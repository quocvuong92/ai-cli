# AI CLI

> A powerful command-line interface for AI interactions with support for multiple providers, web search, and intelligent command execution.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](go.mod)

## Overview

AI CLI provides a unified interface for interacting with multiple AI providers directly from your terminal. Built with Go for performance and reliability, it offers seamless integration with GitHub Copilot and Azure OpenAI, along with advanced features like web search, command execution, and streaming responses.

## Features

### Core Capabilities
- **Multi-Provider Support** - GitHub Copilot (free with Pro) and Azure OpenAI
- **Interactive Chat Mode** - Persistent conversations with command history
- **Streaming Responses** - Real-time output for faster interactions
- **Web Search Integration** - Tavily, Linkup, and Brave Search support
- **Smart Command Execution** - AI-powered terminal operations with safety controls
- **Markdown Rendering** - Beautiful syntax highlighting and formatting

### Advanced Features
- Automatic provider detection based on authentication status
- API key rotation for free tier usage optimization
- Configurable safety levels for command execution
- Session-based command allowlisting
- Model switching on-the-fly
- Citation support for web searches

## Installation

### From Source

```bash
git clone https://github.com/yourusername/ai-cli
cd ai-cli
make build
```

The binary will be available at `bin/ai-cli`.

### Install to System

```bash
make install  # Installs to ~/go/bin/
```

## Quick Start

### GitHub Copilot (Recommended)

```bash
# Authenticate once
ai-cli login

# Start using immediately
ai-cli "Explain Kubernetes architecture"
```

### Azure OpenAI

```bash
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_API_KEY="your-api-key"
export AZURE_OPENAI_MODELS="gpt-4o,gpt-4"
export AI_PROVIDER="azure"
```

## Usage

### Basic Queries

```bash
# Simple question
ai-cli "What is the difference between TCP and UDP?"

# Code generation
ai-cli "Write a binary search function in Go"

# With streaming for immediate feedback
ai-cli -s "Explain how garbage collection works"
```

### Interactive Mode

```bash
# Start interactive session
ai-cli -i

# With all features enabled
ai-cli -sri  # streaming + rendering + interactive
```

#### Interactive Commands

| Command | Description |
|---------|-------------|
| `/web on\|off` | Toggle web search |
| `/model <name>` | Switch AI model |
| `/clear` | Clear conversation history |
| `/allow-dangerous` | Enable risky commands (with confirmation) |
| `/show-permissions` | Display command execution settings |
| `/help` | Show all commands |

### Web Search

Add real-time information to your queries:

```bash
# Enable web search
export TAVILY_API_KEYS="your-key"

# Query with context
ai-cli -w "What are the latest features in Go 1.24?"

# With citations
ai-cli -wc "Current best practices for microservices"
```

### Model Selection

```bash
# List available models
ai-cli --list-models

# Use specific model
ai-cli --model claude-sonnet-4.5 "Complex reasoning task"

# Switch model in interactive mode
> /model gpt-5
```

## Available Models

### GitHub Copilot Free Tier (Pro Subscription)
- **gpt-5-mini** - Lightweight, fast responses (default)
- **gpt-4.1** - Balanced performance and quality
- **grok-code-fast-1** - Optimized for code tasks

### Premium Models
- **GPT Series:** gpt-5, gpt-5.1, gpt-4o, gpt-4o-mini, gpt-4
- **Claude Series:** claude-sonnet-4, claude-sonnet-4.5, claude-opus-4.5, claude-haiku-4.5
- **Gemini Series:** gemini-2.5-pro, gemini-3-pro-preview

## Command Execution

AI CLI can execute terminal commands on your behalf with intelligent safety controls.

### Example Session

```bash
$ ai-cli -i
> List files in the current directory
🔧 Executing: ls -la
total 48
drwxr-xr-x  8 user  staff   256 Dec 10 22:00 .
drwxr-xr-x  5 user  staff   160 Dec 10 21:00 ..
-rw-r--r--  1 user  staff  1234 Dec 10 22:00 README.md

> Create a Python hello world script
⚠️  Command: echo 'print("Hello, World!")' > hello.py
Reason: Creating requested Python script
Allow? [y/n/a]: y
✅ Command executed successfully
```

### Safety Levels

| Level | Examples | Behavior |
|-------|----------|----------|
| 🟢 Safe | `ls`, `cat`, `git status` | Auto-approved |
| 🟡 Moderate | `git commit`, `npm install`, `mkdir` | Requires confirmation |
| 🔴 Dangerous | `rm -rf`, `sudo`, `chmod 777` | Blocked by default |

## Configuration

### Environment Variables

#### Provider Settings
| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AI_PROVIDER` | No | auto-detect | Force provider: `copilot` or `azure` |
| `COPILOT_ACCOUNT_TYPE` | No | individual | Account type: `individual`, `business`, `enterprise` |
| `COPILOT_MODELS` | No | built-in | Comma-separated model list |

#### Azure OpenAI Settings
| Variable | Required | Description |
|----------|----------|-------------|
| `AZURE_OPENAI_ENDPOINT` | For Azure | Your Azure endpoint URL |
| `AZURE_OPENAI_API_KEY` | For Azure | Azure API key |
| `AZURE_OPENAI_MODELS` | No | Available models (comma-separated) |

#### Web Search Settings
| Variable | Required | Description |
|----------|----------|-------------|
| `TAVILY_API_KEYS` | No | Tavily API keys (comma-separated for rotation) |
| `LINKUP_API_KEYS` | No | Linkup API keys (comma-separated) |
| `BRAVE_API_KEYS` | No | Brave Search API keys |
| `WEB_SEARCH_PROVIDER` | No | Default provider: `tavily`, `linkup`, `brave` |

### Command-Line Flags

```
Application Flags:
  -i, --interactive              Interactive chat mode
  -s, --stream                   Stream responses in real-time
  -r, --render                   Render markdown with formatting
  -w, --web                      Enable web search
  -c, --citations                Show source citations
  -m, --model <name>             Select AI model
  -u, --usage                    Display token usage statistics
  -v, --verbose                  Enable debug logging
  
Provider Flags:
      --provider <name>          Force provider (copilot/azure)
      --list-models              List available models
  -p, --search-provider <name>   Web search provider (tavily/linkup/brave)
```

## Commands

### Authentication
```bash
ai-cli login       # Authenticate with GitHub Copilot
ai-cli logout      # Remove stored credentials
ai-cli status      # Show authentication status
```

### Information
```bash
ai-cli --list-models    # Display available models
ai-cli --help           # Show usage information
```

## Building

### Standard Build
```bash
make build              # Build for current platform
```

### Optimized Build
```bash
make build-compressed   # Build with compression (65.9% size reduction)
```

### Cross-Platform Build
```bash
make build-all                # Build for all platforms
make build-all-compressed     # Build and compress for all platforms
```

### Other Make Targets
```bash
make clean             # Remove build artifacts
make install           # Install to ~/go/bin
make tidy              # Update dependencies
make run ARGS="query"  # Run without building
```

## Security

### Authentication
- GitHub tokens stored securely in `~/.local/share/ai-cli/`
- Tokens never transmitted except to official APIs
- Automatic token refresh for Copilot (every ~15 minutes)

### Command Execution
- Pattern-based command classification
- User confirmation required for write operations
- Dangerous commands blocked by default
- 30-second execution timeout
- Session-based allowlist for trusted commands

## Examples

### Development Workflow
```bash
# Generate code
ai-cli "Create a REST API handler for user authentication"

# Debug issues
ai-cli -w "Why am I getting CORS errors in my React app?"

# Code review
ai-cli "Review this function for potential issues: $(cat myfile.go)"
```

### System Administration
```bash
# Troubleshooting
ai-cli "How to check disk usage on Linux?"

# Interactive system management
ai-cli -i
> Show disk usage
> List running Docker containers
> Check system memory
```

### Research
```bash
# Latest information
ai-cli -wc "What are the new features in Kubernetes 1.29?"

# Technical comparison
ai-cli --model claude-opus-4.5 "Compare PostgreSQL vs MongoDB"
```

## Troubleshooting

### Common Issues

**"Not logged in" error**
```bash
ai-cli login
```

**"No provider configured" error**
```bash
# Set up GitHub Copilot or Azure OpenAI environment variables
```

**Models not showing**
```bash
ai-cli --list-models
# Ensure you're logged in or have Azure credentials set
```

**Command execution not working**
```bash
ai-cli -i
> /show-permissions
> /allow-dangerous  # If needed, use with caution
```

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Resources

- [GitHub Copilot Documentation](https://docs.github.com/en/copilot)
- [Azure OpenAI Service](https://learn.microsoft.com/en-us/azure/ai-services/openai/)
- [Implementation Details](PLAN.md)

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Glamour](https://github.com/charmbracelet/glamour) - Markdown rendering
- [Go Prompt](https://github.com/elk-language/go-prompt) - Interactive terminal

---

**Made with ❤️ by developers, for developers**
