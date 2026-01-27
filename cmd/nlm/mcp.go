package main

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/nlm/internal/api"
)

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

	if debug {
		fmt.Fprintf(os.Stderr, "nlm: starting MCP server...\n")
	}

	return server.Run(context.Background(), &mcp.StdioTransport{})
}

func isURL(input string) bool {
	return len(input) > 8 && (input[:8] == "https://" || input[:7] == "http://")
}
