package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/nlm/internal/api"
	"github.com/tmc/nlm/internal/auth"
	"github.com/tmc/nlm/internal/batchexecute"
)

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// mcpClientManager handles automatic re-authentication for the MCP server.
// When an API call fails with an auth error, it refreshes credentials via
// headless Chrome and rebuilds the client transparently.
type mcpClientManager struct {
	mu     sync.Mutex
	client *api.Client
	opts   []batchexecute.Option
}

func newMCPClientManager(client *api.Client, opts ...batchexecute.Option) *mcpClientManager {
	return &mcpClientManager{client: client, opts: opts}
}

func (m *mcpClientManager) getClient() *api.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client
}

// refreshAuthFromDisk re-reads ~/.nlm/env and rebuilds the client if the
// credentials differ from what's currently loaded.
// Returns true if creds were updated.
func (m *mcpClientManager) refreshAuthFromDisk() bool {
	token, cookies, ok := readEnvFile()
	if !ok {
		return false
	}
	m.client = api.New(token, cookies, m.opts...)
	fmt.Fprintf(os.Stderr, "nlm-mcp: reloaded credentials from ~/.nlm/env\n")
	return true
}

// refreshAuthViaBrowser launches headless Chrome to get fresh credentials.
// In MCP mode stdout is the JSON-RPC transport, so we redirect it to stderr.
func (m *mcpClientManager) refreshAuthViaBrowser() error {
	fmt.Fprintf(os.Stderr, "nlm-mcp: attempting headless browser auth...\n")
	origStdout := os.Stdout
	os.Stdout = os.Stderr
	defer func() { os.Stdout = origStdout }()

	a := auth.New(debug)
	token, cookies, err := a.GetAuth(
		auth.WithScanBeforeAuth(),
		auth.WithProfileName(chromeProfile),
	)
	if err != nil {
		return fmt.Errorf("headless auth failed (run 'nlm auth' manually): %w", err)
	}

	os.Stdout = origStdout // restore before saving

	if saveErr := saveCredentials(token, cookies); saveErr != nil {
		fmt.Fprintf(os.Stderr, "nlm-mcp: warning: failed to save credentials: %v\n", saveErr)
	}
	m.client = api.New(token, cookies, m.opts...)
	fmt.Fprintf(os.Stderr, "nlm-mcp: credentials refreshed via browser\n")
	return nil
}

// readEnvFile reads auth credentials directly from ~/.nlm/env, bypassing
// the environment variable cache (which loadStoredEnv skips if already set).
func readEnvFile() (token, cookies string, ok bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", false
	}
	data, err := os.ReadFile(filepath.Join(home, ".nlm", "env"))
	if err != nil {
		return "", "", false
	}
	s := bufio.NewScanner(strings.NewReader(string(data)))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		switch key {
		case "NLM_AUTH_TOKEN":
			token = value
		case "NLM_COOKIES":
			cookies = value
		}
	}
	return token, cookies, token != "" && cookies != ""
}

// call executes fn with the current client. On auth error it tries two
// refresh strategies: 1) reload from ~/.nlm/env (instant), 2) headless Chrome.
func (m *mcpClientManager) call(fn func(c *api.Client) (*mcp.CallToolResult, any, error)) (*mcp.CallToolResult, any, error) {
	result, extra, err := fn(m.getClient())
	if err == nil || !isAuthenticationError(err) {
		return result, extra, err
	}

	fmt.Fprintf(os.Stderr, "nlm-mcp: authentication expired, refreshing credentials...\n")

	m.mu.Lock()
	defer m.mu.Unlock()

	// Strategy 1: disk creds may have been refreshed by another process.
	if m.refreshAuthFromDisk() {
		result, extra, err = fn(m.client)
		if err == nil || !isAuthenticationError(err) {
			return result, extra, err
		}
		fmt.Fprintf(os.Stderr, "nlm-mcp: disk credentials also expired, trying browser...\n")
	}

	// Strategy 2: headless Chrome auth.
	if refreshErr := m.refreshAuthViaBrowser(); refreshErr != nil {
		return nil, nil, fmt.Errorf("re-authentication failed: %w (original error: %v)", refreshErr, err)
	}

	return fn(m.client)
}

func runMCPServer(client *api.Client) error {
	mgr := newMCPClientManager(client)
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "nlm",
		Version: "0.1.0",
	}, nil)

	// List Notebooks
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_notebooks",
		Description: "List all NotebookLM notebooks",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			projects, err := c.ListRecentlyViewedProjects()
			if err != nil {
				return nil, nil, err
			}
			var res string
			if len(projects) == 0 {
				res = "No notebooks found."
			} else {
				for _, p := range projects {
					res += fmt.Sprintf("- %s (%s)\n", p.Title, p.ProjectId)
				}
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: res},
				},
			}, nil, nil
		})
	})

	// Create Notebook
	type createArgs struct {
		Title string `json:"title" jsonschema:"The title of the new notebook"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_notebook",
		Description: "Create a new NotebookLM notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args createArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			project, err := c.CreateProject(args.Title, "")
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Created notebook: %s (%s)", project.Title, project.ProjectId)},
				},
			}, nil, nil
		})
	})

	// List Sources
	type listSourcesArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sources",
		Description: "List all sources in a notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args listSourcesArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			project, err := c.GetProject(args.NotebookID)
			if err != nil {
				return nil, nil, err
			}
			var res string
			if len(project.Sources) == 0 {
				res = "No sources found in this notebook."
			} else {
				for _, s := range project.Sources {
					sourceID := ""
					if sid := s.GetSourceId(); sid != nil {
						sourceID = sid.GetSourceId()
					}
					sourceType := "unknown"
					if meta := s.GetMetadata(); meta != nil {
						sourceType = meta.GetSourceType().String()
					}
					res += fmt.Sprintf("- %s (%s) [type: %s]\n", s.Title, sourceID, sourceType)
				}
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: res},
				},
			}, nil, nil
		})
	})

	// Add Source
	type addSourceArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		Input      string `json:"input" jsonschema:"File path, URL, or '-' for stdin content"`
		Content    string `json:"content,omitempty" jsonschema:"Optional text content if input is '-' or to be added directly"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_source",
		Description: "Add a source to a notebook (URL or File path)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args addSourceArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			var sourceID string
			var err error
			if args.Input == "-" || args.Content != "" {
				content := args.Content
				sourceID, err = c.AddSourceFromText(args.NotebookID, content, "New Source")
			} else if isURL(args.Input) {
				sourceID, err = c.AddSourceFromURL(args.NotebookID, args.Input)
			} else {
				sourceID, err = c.AddSourceFromFile(args.NotebookID, args.Input)
			}
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Added source: %s", sourceID)},
				},
			}, nil, nil
		})
	})

	// Get Source
	type getSourceArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		SourceID   string `json:"source_id" jsonschema:"The ID of the source"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_source",
		Description: "Get metadata about a source. Note: full content retrieval is not currently available via the API; use generate_chat to ask questions about source content.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args getSourceArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			// LoadSource RPC (hizoJc) was deprecated by Google.
			// Fall back to finding the source in the project's source list.
			project, err := c.GetProject(args.NotebookID)
			if err != nil {
				return nil, nil, err
			}
			for _, s := range project.Sources {
				sid := ""
				if id := s.GetSourceId(); id != nil {
					sid = id.GetSourceId()
				}
				if sid == args.SourceID {
					res := fmt.Sprintf("Title: %s\nID: %s\n", s.Title, sid)
					if meta := s.GetMetadata(); meta != nil {
						res += fmt.Sprintf("Type: %s\n", meta.GetSourceType())
						if meta.LastModifiedTime != nil {
							res += fmt.Sprintf("Last Modified: %s\n", meta.LastModifiedTime.AsTime().Format("2006-01-02T15:04:05Z"))
						}
					}
					res += "\nNote: Full source content is not available via the API. Use generate_chat to ask questions grounded in this source."
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: res},
						},
					}, nil, nil
				}
			}
			return nil, nil, fmt.Errorf("source %s not found in notebook", args.SourceID)
		})
	})

	// List Notes
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_notes",
		Description: "List all notes in a notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args listSourcesArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			notes, err := c.GetNotes(args.NotebookID)
			if err != nil {
				return nil, nil, err
			}
			var res string
			if len(notes) == 0 {
				res = "No notes found in this notebook."
			} else {
				for _, n := range notes {
					noteID := ""
					if sid := n.GetSourceId(); sid != nil {
						noteID = sid.GetSourceId()
					}
					res += fmt.Sprintf("- %s (%s)\n", n.Title, noteID)
				}
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: res},
				},
			}, nil, nil
		})
	})

	// Get Note
	type getNoteArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		NoteID     string `json:"note_id" jsonschema:"The ID of the note"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_note",
		Description: "Get a note's title and content",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args getNoteArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			note, err := c.GetNote(args.NotebookID, args.NoteID)
			if err != nil {
				return nil, nil, err
			}
			res := fmt.Sprintf("Title: %s\n", note.Title)
			noteID := ""
			if sid := note.GetSourceId(); sid != nil {
				noteID = sid.GetSourceId()
			}
			res += fmt.Sprintf("ID: %s\n", noteID)
			if note.Content != "" {
				res += fmt.Sprintf("\n%s", note.Content)
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: res},
				},
			}, nil, nil
		})
	})

	// Create Note
	type createNoteArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		Title      string `json:"title" jsonschema:"The title of the note"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_note",
		Description: "Create a new note in a notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args createNoteArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			note, err := c.CreateNote(args.NotebookID, args.Title, "")
			if err != nil {
				return nil, nil, err
			}
			noteID := ""
			if note != nil {
				if sid := note.GetSourceId(); sid != nil {
					noteID = sid.GetSourceId()
				}
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Created note: %s (ID: %s)", args.Title, noteID)},
				},
			}, nil, nil
		})
	})

	// Generate Chat
	type generateChatArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		Prompt     string `json:"prompt" jsonschema:"The prompt to generate a response for"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate_chat",
		Description: "Generate a response from NotebookLM based on sources",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args generateChatArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			response, err := c.GenerateFreeFormStreamed(args.NotebookID, args.Prompt, nil)
			if err != nil {
				return nil, nil, err
			}
			res := response.Chunk
			if res == "" {
				res = "(No response received)"
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: res},
				},
			}, nil, nil
		})
	})

	// Import Research Sources
	type importResearchArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		TaskID     string `json:"task_id" jsonschema:"The task ID returned by run_deep_research"`
		URLs       string `json:"urls" jsonschema:"Use 'all' to import all discovered sources, or provide a comma-separated list of specific URLs to import"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "import_research_sources",
		Description: "Import discovered research sources into the notebook. Call after run_deep_research completes (with auto_import=false) to selectively import sources. Use urls='all' to import everything, or specify individual URLs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args importResearchArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			var sources []api.DiscoveredSource

			if strings.EqualFold(strings.TrimSpace(args.URLs), "all") {
				pollResult, err := c.PollResearch(args.NotebookID)
				if err != nil {
					return nil, nil, fmt.Errorf("poll for sources: %w", err)
				}
				if pollResult.State != api.ResearchStateCompleted {
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: fmt.Sprintf("Research is not yet complete (status: %s). Wait and try again.", pollResult.State)},
						},
						IsError: true,
					}, nil, nil
				}
				sources = pollResult.Sources
				if args.TaskID == "" {
					args.TaskID = pollResult.TaskID
				}
			} else {
				for _, u := range splitAndTrim(args.URLs) {
					if u != "" {
						sources = append(sources, api.DiscoveredSource{URL: u, Title: u})
					}
				}
			}

			var importable []api.DiscoveredSource
			for _, s := range sources {
				if s.URL != "" {
					importable = append(importable, s)
				}
			}
			if len(importable) == 0 {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: "No sources with URLs available to import. Deep research sources often lack URLs. The research summary was returned by run_deep_research."},
					},
					IsError: true,
				}, nil, nil
			}

			imported, err := c.ImportResearchSources(args.NotebookID, args.TaskID, importable)
			if err != nil {
				return nil, nil, err
			}
			res := fmt.Sprintf("Imported %d source(s):\n", len(imported))
			for _, s := range imported {
				res += fmt.Sprintf("  - %s (ID: %s)\n", s.Title, s.ID)
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: res},
				},
			}, nil, nil
		})
	})

	// Run Deep Research (blocking convenience)
	type runDeepResearchArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		Query      string `json:"query" jsonschema:"The research topic or question to investigate"`
		AutoImport bool   `json:"auto_import,omitempty" jsonschema:"If true, automatically import URL-bearing sources after research completes. Deep research sources often lack URLs so import may be partial. Default: false"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_deep_research",
		Description: "Run deep research and wait for completion. This typically takes 5-10 minutes. The tool polls internally and returns when finished or after 10 minutes. Returns discovered sources and a summary. Set auto_import=true to automatically import all discovered sources, or leave it false and use import_research_sources afterwards to selectively import specific URLs.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args runDeepResearchArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			var result *api.ResearchResult
			var err error
			if args.AutoImport {
				result, err = c.RunDeepResearchAndImport(ctx, args.NotebookID, args.Query)
			} else {
				result, err = c.RunDeepResearch(ctx, args.NotebookID, args.Query)
			}
			if err != nil {
				if result != nil && result.TaskID != "" {
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: fmt.Sprintf("Deep research error (task_id=%s): %v", result.TaskID, err)},
						},
						IsError: true,
					}, nil, nil
				}
				return nil, nil, err
			}
			res := formatResearchResult(result, nil)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: res},
				},
			}, nil, nil
		})
	})

	// Research and Import (convenience: fast research + auto-import)
	type researchAndImportArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		Query      string `json:"query" jsonschema:"The search query to find relevant sources"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "research_and_import",
		Description: "Run fast web research and automatically import discovered sources into the notebook. Blocks until research completes (typically 30-60 seconds).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args researchAndImportArgs) (*mcp.CallToolResult, any, error) {
		return mgr.call(func(c *api.Client) (*mcp.CallToolResult, any, error) {
			result, err := c.RunFastResearch(ctx, args.NotebookID, args.Query, true)
			if err != nil {
				if result != nil && result.TaskID != "" {
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: fmt.Sprintf("Research error (task_id=%s): %v", result.TaskID, err)},
						},
						IsError: true,
					}, nil, nil
				}
				return nil, nil, err
			}
			res := formatResearchResult(result, nil)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: res},
				},
			}, nil, nil
		})
	})

	if debug {
		fmt.Fprintf(os.Stderr, "nlm: starting MCP server...\n")
	}

	return server.Run(context.Background(), &mcp.StdioTransport{})
}

func isURL(input string) bool {
	return len(input) > 8 && (input[:8] == "https://" || input[:7] == "http://")
}

// formatResearchResult formats a ResearchResult into a human-readable string.
// importErr is optional — if non-nil, it indicates that import was attempted but failed.
func formatResearchResult(result *api.ResearchResult, importErr error) string {
	res := fmt.Sprintf("Query: %s\nStatus: %s\n", result.Query, result.State)
	if result.TaskID != "" {
		res += fmt.Sprintf("Task ID: %s\n", result.TaskID)
	}

	if len(result.Sources) > 0 {
		importable := 0
		for _, s := range result.Sources {
			if s.URL != "" {
				importable++
			}
		}
		res += fmt.Sprintf("\nFound %d sources (%d with URLs):\n", len(result.Sources), importable)
		for i, s := range result.Sources {
			if s.URL != "" {
				res += fmt.Sprintf("  %d. %s (%s)\n", i+1, s.Title, s.URL)
			} else {
				res += fmt.Sprintf("  %d. %s [no URL]\n", i+1, s.Title)
			}
			if s.Description != "" {
				res += fmt.Sprintf("     %s\n", s.Description)
			}
		}
	}

	if result.Summary != "" {
		res += fmt.Sprintf("\nSummary:\n%s\n", result.Summary)
	}

	if len(result.Imported) > 0 {
		res += fmt.Sprintf("\nImported %d source(s):\n", len(result.Imported))
		for _, s := range result.Imported {
			res += fmt.Sprintf("  - %s (ID: %s)\n", s.Title, s.ID)
		}
	} else if importErr != nil {
		res += fmt.Sprintf("\nImport failed: %v\n", importErr)
	}

	return res
}
