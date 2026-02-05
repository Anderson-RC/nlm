package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/nlm/internal/api"
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

func runMCPServer(client *api.Client) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "nlm",
		Version: "0.1.0",
	}, nil)

	// List Notebooks
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_notebooks",
		Description: "List all NotebookLM notebooks",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		projects, err := client.ListRecentlyViewedProjects()
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

	// Create Notebook
	type createArgs struct {
		Title string `json:"title" jsonschema:"The title of the new notebook"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_notebook",
		Description: "Create a new NotebookLM notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args createArgs) (*mcp.CallToolResult, any, error) {
		project, err := client.CreateProject(args.Title, "")
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Created notebook: %s (%s)", project.Title, project.ProjectId)},
			},
		}, nil, nil
	})

	// List Sources
	type listSourcesArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sources",
		Description: "List all sources in a notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args listSourcesArgs) (*mcp.CallToolResult, any, error) {
		project, err := client.GetProject(args.NotebookID)
		if err != nil {
			return nil, nil, err
		}
		var res string
		if len(project.Sources) == 0 {
			res = "No sources found in this notebook."
		} else {
			for _, s := range project.Sources {
				sourceID := s.GetSourceId().GetSourceId()
				res += fmt.Sprintf("- %s (%s) [type: %s]\n", s.Title, sourceID, s.GetMetadata().GetSourceType())
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: res},
			},
		}, nil, nil
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
		var sourceID string
		var err error
		if args.Input == "-" || args.Content != "" {
			content := args.Content
			sourceID, err = client.AddSourceFromText(args.NotebookID, content, "New Source")
		} else if isURL(args.Input) {
			sourceID, err = client.AddSourceFromURL(args.NotebookID, args.Input)
		} else {
			sourceID, err = client.AddSourceFromFile(args.NotebookID, args.Input)
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

	// Get Source
	type getSourceArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		SourceID   string `json:"source_id" jsonschema:"The ID of the source"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_source",
		Description: "Get the content of a source",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args getSourceArgs) (*mcp.CallToolResult, any, error) {
		source, err := client.LoadSource(args.SourceID)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: source.Content},
			},
		}, nil, nil
	})

	// List Notes
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_notes",
		Description: "List all notes in a notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args listSourcesArgs) (*mcp.CallToolResult, any, error) {
		notes, err := client.GetNotes(args.NotebookID)
		if err != nil {
			return nil, nil, err
		}
		var res string
		if len(notes) == 0 {
			res = "No notes found in this notebook."
		} else {
			for _, n := range notes {
				sourceID := n.GetSourceId().GetSourceId()
				res += fmt.Sprintf("- %s (%s)\n", n.Title, sourceID)
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: res},
			},
		}, nil, nil
	})

	// Get Note
	type getNoteArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		NoteID     string `json:"note_id" jsonschema:"The ID of the note"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_note",
		Description: "Get the content of a note",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args getNoteArgs) (*mcp.CallToolResult, any, error) {
		source, err := client.GetNote(args.NotebookID, args.NoteID)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: source.Content},
			},
		}, nil, nil
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
		_, err := client.CreateNote(args.NotebookID, args.Title, "")
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Created note: %s", args.Title)},
			},
		}, nil, nil
	})

	// Delete Notebook
	type deleteNotebookArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook to delete"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_notebook",
		Description: "Delete a NotebookLM notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args deleteNotebookArgs) (*mcp.CallToolResult, any, error) {
		err := client.DeleteProjects([]string{args.NotebookID})
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Deleted notebook: %s", args.NotebookID)},
			},
		}, nil, nil
	})

	// Delete Source
	type deleteSourceArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		SourceID   string `json:"source_id" jsonschema:"The ID of the source to delete"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_source",
		Description: "Delete a source from a notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args deleteSourceArgs) (*mcp.CallToolResult, any, error) {
		err := client.DeleteSources(args.NotebookID, []string{args.SourceID})
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Deleted source: %s", args.SourceID)},
			},
		}, nil, nil
	})

	// Delete Note
	type deleteNoteArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		NoteID     string `json:"note_id" jsonschema:"The ID of the note to delete"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_note",
		Description: "Delete a note from a notebook",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args deleteNoteArgs) (*mcp.CallToolResult, any, error) {
		err := client.DeleteNotes(args.NotebookID, []string{args.NoteID})
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Deleted note: %s", args.NoteID)},
			},
		}, nil, nil
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
		response, err := client.GenerateFreeFormStreamed(args.NotebookID, args.Prompt, nil)
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
		var sources []api.DiscoveredSource

		if strings.EqualFold(strings.TrimSpace(args.URLs), "all") {
			// Poll to get all sources, then import them all
			pollResult, err := client.PollResearch(args.NotebookID)
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
			// Use the task ID from poll if not provided
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

		// Filter to only sources with URLs (required for import)
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

		imported, err := client.ImportResearchSources(args.NotebookID, args.TaskID, importable)
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
		var result *api.ResearchResult
		var err error
		if args.AutoImport {
			result, err = client.RunDeepResearchAndImport(ctx, args.NotebookID, args.Query)
		} else {
			result, err = client.RunDeepResearch(ctx, args.NotebookID, args.Query)
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

	// Research and Import (convenience: fast research + auto-import)
	type researchAndImportArgs struct {
		NotebookID string `json:"notebook_id" jsonschema:"The ID of the notebook"`
		Query      string `json:"query" jsonschema:"The search query to find relevant sources"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "research_and_import",
		Description: "Run fast web research and automatically import discovered sources into the notebook. Blocks until research completes (typically 30-60 seconds).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args researchAndImportArgs) (*mcp.CallToolResult, any, error) {
		result, err := client.RunFastResearch(ctx, args.NotebookID, args.Query, true)
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
