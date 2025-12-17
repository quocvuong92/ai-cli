# AI CLI

A fast, lightweight command-line interface for AI interactions. Supports GitHub Copilot and Azure OpenAI with web search integration.

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Features

- **Multiple AI Providers** - GitHub Copilot (free with Pro) and Azure OpenAI
- **Web Search** - Tavily, Linkup, and Brave Search integration
- **Interactive Mode** - Persistent conversations with command execution
- **Streaming** - Real-time response output
- **Smart Permissions** - Three-tier command safety system

## Installation

```bash
git clone https://github.com/quocvuong92/ai-cli
cd ai-cli
make install
```

## Quick Start

```bash
# Login to GitHub Copilot
ai-cli login

# Ask a question
ai-cli "Explain Docker containers"

# Interactive mode with streaming
ai-cli -si

# With web search
ai-cli -w "Latest Go 1.24 features"
```

## Configuration

### Config File

Create `~/.config/ai-cli/config.yaml`:

```yaml
provider: copilot
model: gpt-4.1

copilot:
  account_type: individual
  models:
    - gpt-4.1
    - gpt-5.1
    - claude-sonnet-4.5

web_search:
  provider: tavily
  tavily_keys:
    - your-api-key

defaults:
  stream: true
  render: false
```

See [config.example.yaml](config.example.yaml) for all options.

### Environment Variables

Environment variables override config file settings:

```bash
# Provider
AI_PROVIDER=copilot|azure

# Azure OpenAI
AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com
AZURE_OPENAI_API_KEY=your-key

# Web Search (comma-separated for key rotation)
TAVILY_API_KEYS=key1,key2
LINKUP_API_KEYS=key1
BRAVE_API_KEYS=key1
```

## Usage

### Command Line

```bash
ai-cli [flags] "your query"

Flags:
  -i, --interactive    Interactive chat mode
  -s, --stream         Stream responses
  -r, --render         Render markdown
  -w, --web            Enable web search
  -c, --citations      Show sources
  -m, --model          Select model
  -v, --verbose        Debug logging
      --list-models    List available models
```

### Interactive Commands

| Command | Description |
|---------|-------------|
| `/web on\|off` | Toggle web search |
| `/model <name>` | Switch model |
| `/clear` | Clear history |
| `/allow-dangerous` | Enable risky commands |
| `/show-permissions` | View permission settings |

### Command Execution

Commands are classified by safety level:

| Level | Examples | Behavior |
|-------|----------|----------|
| Safe | `ls`, `cat`, `git status` | Auto-approved |
| Moderate | `git commit`, `mkdir` | Requires confirmation |
| Dangerous | `rm -rf`, `sudo` | Blocked by default |

Permission options: `[y]es once` / `[s]ession` / `[a]lways` / `[n]o`

## Commands

```bash
ai-cli login       # Authenticate with GitHub Copilot
ai-cli logout      # Remove credentials
ai-cli status      # Show auth status
```

## Build

```bash
make build         # Build binary
make install       # Install to ~/go/bin
make test          # Run tests
make clean         # Clean artifacts
```

## License

MIT License - see [LICENSE](LICENSE)
