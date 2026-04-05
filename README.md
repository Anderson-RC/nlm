# nlm – NotebookLM CLI & MCP Server

`nlm` is a command-line tool and MCP server for Google NotebookLM. Manage notebooks, sources, and notes, run research, chat against your sources, and generate structured content – all without opening a browser.

## Installation

```bash
go install github.com/tmc/nlm/cmd/nlm@latest
```

<details>
<summary>Installing Go (if needed)</summary>

### Package Managers

**macOS (Homebrew):**
```bash
brew install go
```

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install golang
```

**Fedora:**
```bash
sudo dnf install golang
```

### Direct Download

1. Visit the [Go downloads page](https://go.dev/dl/)
2. Download the appropriate version for your OS and follow the installer

### Post-installation

Add Go to your PATH in `~/.bashrc`, `~/.zshrc`, or equivalent:
```bash
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$(go env GOPATH)/bin
```

Verify with:
```bash
go version
```
</details>

## Authentication

```bash
nlm auth
```

This launches a Chromium-based browser to authenticate with your Google account. Tokens are saved to `~/.nlm/env`.

Supported browsers: Chrome, Chrome Canary, Brave, Chromium, and Microsoft Edge. The tool scans for available profiles and prioritises those that already have NotebookLM cookies.

```bash
nlm auth --profile "Work Profile"   # Use a specific profile
nlm auth --all                      # Try all available profiles
nlm auth --notebooks                # Show which profiles have NotebookLM access
nlm auth --debug                    # Verbose output
```

### Environment Variables

These are managed by `nlm auth` but can be set manually if needed:

- `NLM_AUTH_TOKEN` – authentication token (stored in `~/.nlm/env`)
- `NLM_COOKIES` – authentication cookies (stored in `~/.nlm/env`)
- `NLM_BROWSER_PROFILE` – browser profile name (default: `"Default"`)

## CLI Usage

### Notebooks

```bash
nlm list                        # List all notebooks
nlm create "My Research"        # Create a notebook
nlm rm <notebook-id>            # Delete a notebook
nlm analytics <notebook-id>     # Show notebook analytics
```

### Sources

Sources can be URLs, local files, or piped text. YouTube URLs are detected automatically and added as video sources.

```bash
nlm sources <notebook-id>                     # List sources
nlm add <notebook-id> https://example.com     # Add URL source
nlm add <notebook-id> document.pdf            # Add file source
echo "text" | nlm add <notebook-id> -         # Add from stdin
nlm add <notebook-id> - -mime="text/xml"      # Stdin with explicit MIME type
nlm rm-source <notebook-id> <source-id>       # Remove source
nlm rename-source <source-id> "New Title"     # Rename source
nlm refresh-source <source-id>                # Refresh source content
nlm check-source <source-id>                  # Check freshness
nlm discover-sources <notebook-id>            # Discover relevant sources
```

### Notes

```bash
nlm notes <notebook-id>                                    # List notes
nlm read-note <notebook-id> <note-id>                      # Read note content
nlm read-note <notebook-id> <note-id> --full               # Read with full details
nlm new-note <notebook-id> "Title"                         # Create note
nlm update-note <notebook-id> <note-id> "New content"      # Update note
nlm rm-note <note-id>                                      # Remove note
```

### Chat and Generation

`chat` opens an interactive session grounded in your notebook's sources. The generation commands produce structured content in a single call.

```bash
nlm chat <notebook-id>                    # Interactive chat session
nlm generate-chat <notebook-id> "prompt"  # One-shot chat
nlm generate-guide <notebook-id>          # Generate notebook guide
nlm generate-outline <notebook-id>        # Generate content outline
nlm generate-section <notebook-id>        # Generate new section
nlm generate-mindmap <notebook-id>        # Generate mindmap
```

### Content Transformation

These commands take your notebook's sources and produce a particular kind of output.

```bash
nlm summarize <notebook-id>       # Summarize sources
nlm explain <notebook-id>         # Explain concepts
nlm critique <notebook-id>        # Critique content
nlm brainstorm <notebook-id>      # Brainstorm ideas
nlm expand <notebook-id>          # Expand on content
nlm rephrase <notebook-id>        # Rephrase content
nlm verify <notebook-id>          # Verify facts
nlm outline <notebook-id>         # Create outline
nlm study-guide <notebook-id>     # Generate study guide
nlm faq <notebook-id>             # Generate FAQ
nlm briefing-doc <notebook-id>    # Create briefing document
nlm timeline <notebook-id>        # Create timeline
nlm toc <notebook-id>             # Generate table of contents
```

### Audio and Video

```bash
nlm audio-create <notebook-id> "instructions"   # Create audio overview
nlm audio-get <notebook-id>                      # Get audio overview
nlm audio-download <notebook-id>                 # Download audio file
nlm audio-share <notebook-id>                    # Share audio (private)
nlm audio-share <notebook-id> --public           # Share audio (public)
nlm audio-rm <notebook-id>                       # Delete audio overview

nlm video-create <notebook-id>                   # Create video overview
nlm video-download <notebook-id>                 # Download video file
```

### Sharing

```bash
nlm share <notebook-id>              # Share notebook publicly
nlm share-private <notebook-id>      # Share notebook privately
nlm share-details <notebook-id>      # Get sharing details
```

### Debug Mode

Add `-debug` to any command to see the underlying API calls:

```bash
nlm -debug list
```

## MCP Server

`nlm` can run as an MCP server, letting AI assistants like Claude interact with NotebookLM through tool calls.

```bash
nlm mcp
```

### Configuration

Add the following to your MCP client config (e.g. `.mcp.json` for Claude Code):

```json
{
  "mcpServers": {
    "nlm": {
      "command": "nlm",
      "args": ["mcp"]
    }
  }
}
```

### Available Tools

| Tool | Description |
|------|-------------|
| `list_notebooks` | List all notebooks |
| `create_notebook` | Create a new notebook |
| `list_sources` | List sources in a notebook |
| `add_source` | Add a source (URL, file, or text) |
| `get_source` | Get source metadata |
| `list_notes` | List notes in a notebook |
| `get_note` | Get note title and content |
| `create_note` | Create a new note |
| `generate_chat` | Chat with notebook sources |
| `run_deep_research` | Deep research – blocks for 5 – 10 minutes, polls internally |
| `research_and_import` | Fast web research with auto-import – blocks for 30 – 60 seconds |
| `import_research_sources` | Selectively import sources after deep research |

> **Note:** Delete operations are not exposed via MCP to prevent accidental data loss. Use the CLI or the NotebookLM web interface for deletions.

### Research Workflow

**Fast research** runs a quick web search and imports all discovered sources automatically:

```bash
# Via MCP: research_and_import(notebook_id, query)
```

**Deep research** investigates a topic and returns a list of discovered sources. Review the results and choose which to import:

```bash
# Via MCP:
# 1. run_deep_research(notebook_id, query, auto_import=false)
# 2. Review the returned sources
# 3. import_research_sources(notebook_id, task_id, urls="https://example.com,https://other.com")
```

Set `auto_import=true` on `run_deep_research` to skip the selection step and import everything.

## Contributing

Pull requests are welcome.

## Licence

MIT – see [LICENSE](LICENSE) for details.
