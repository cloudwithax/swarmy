<p align="center">
  <img src="./logo.png" alt="Swarmy logo" width="320">
</p>

<h1 align="center">Swarmy</h1>

<p align="center"><strong>Where agents go to fly.</strong></p>

<p align="center">
  A beautiful orchestration layer for agentic swarm coding based on
  <a href="https://github.com/charmbracelet/crush">Crush</a>, from
  <a href="https://charm.land">Charm</a>.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/cloudwithax/swarmy">
    <img src="https://pkg.go.dev/badge/github.com/cloudwithax/swarmy.svg" alt="Go Reference">
  </a>
  <a href="https://goreportcard.com/report/github.com/cloudwithax/swarmy">
    <img src="https://goreportcard.com/badge/github.com/cloudwithax/swarmy" alt="Go Report Card">
  </a>
  <a href="./LICENSE">
    <img src="https://img.shields.io/badge/license-FSL--1.1--MIT-0f766e" alt="License">
  </a>
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-0f766e" alt="Platforms">
</p>

## Features

- **Multi-Model:** choose from a wide range of LLMs or add your own via OpenAI- or Anthropic-compatible APIs
- **Flexible:** switch LLMs mid-session while preserving context
- **Session-Based:** maintain multiple work sessions and contexts per project
- **Swarm Mode:** the agent decides how to delegate work across multiple workers by default based on file-level context
- **LSP-Enhanced:** uses LSPs for additional context, just like you do
- **Extensible:** add capabilities via MCPs (`http`, `stdio`, and `sse`)
- **Headless ACP:** expose Swarmy over ACP so external orchestrators can discover and run it without the TUI
- **Works Everywhere:** macOS, Linux, Windows (PowerShell and WSL), Android, and FreeBSD

## Installation

```
go install github.com/cloudwithax/swarmy@latest
```

## Getting Started

Grab an API key for your preferred provider (Anthropic, OpenAI, Groq, OpenRouter, etc.) and run `swarmy`. You'll be prompted to enter your API key.

You can also set environment variables:

| Environment Variable    | Provider       |
| ----------------------- | -------------- |
| `ANTHROPIC_API_KEY`     | Anthropic      |
| `OPENAI_API_KEY`        | OpenAI         |
| `GEMINI_API_KEY`        | Google Gemini  |
| `GROQ_API_KEY`          | Groq           |
| `OPENROUTER_API_KEY`    | OpenRouter     |
| `AWS_ACCESS_KEY_ID`     | Amazon Bedrock |
| `AWS_SECRET_ACCESS_KEY` | Amazon Bedrock |
| `AWS_REGION`            | Amazon Bedrock |

## Configuration

Configuration can be added locally or globally, with the following priority:

1. `.swarmy.json`
2. `swarmy.json`
3. `$HOME/.config/swarmy/swarmy.json`

### Custom Providers

Supports OpenAI-compatible and Anthropic-compatible APIs:

```json
{
  "providers": {
    "deepseek": {
      "type": "openai-compat",
      "base_url": "https://api.deepseek.com/v1",
      "api_key": "$DEEPSEEK_API_KEY",
      "models": [
        {
          "id": "deepseek-chat",
          "name": "Deepseek V3",
          "context_window": 64000,
          "default_max_tokens": 5000
        }
      ]
    }
  }
}
```

### Local Models

```json
{
  "providers": {
    "ollama": {
      "name": "Ollama",
      "base_url": "http://localhost:11434/v1/",
      "type": "openai-compat",
      "models": [
        {
          "name": "Qwen 3 30B",
          "id": "qwen3:30b",
          "context_window": 256000,
          "default_max_tokens": 20000
        }
      ]
    }
  }
}
```

### LSPs

```json
{
  "lsp": {
    "go": { "command": "gopls" },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    }
  }
}
```

### MCPs

Supports `stdio`, `http`, and `sse` transports:

```json
{
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"]
    }
  }
}
```

### Permissions

```json
{
  "permissions": {
    "allowed_tools": ["view", "ls", "grep", "edit"]
  }
}
```

Or skip all permission prompts with `--yolo`.

### Swarm Architecture

Swarmy defaults to a file-level swarm mode for cost-insensitive workflows. In this mode, the main coder agent can delegate to a swarm planner that selects files and then launches one worker agent per file. In the TUI model picker, use `ctrl+a` to toggle between `swarm` and `solo`.

```json
{
  "options": {
    "agent_architecture": "swarm",
    "swarm": {
      "enabled": true,
      "max_files": 8,
      "max_concurrent_workers": 4
    }
  }
}
```

Set `agent_architecture` to `solo` if you want the original single-agent flow.

## Logging

Logs are stored in `./.swarmy/logs/swarmy.log`.

```bash
swarmy logs
swarmy logs --tail 500
swarmy logs --follow
```

## ACP

Start a headless ACP server on localhost:

```bash
swarmy acp serve
```

This exposes a single ACP agent named `swarmy` on port `8000`. By default the ACP server auto-approves tool permissions for ACP-created runs so external orchestrators can execute tasks without an interactive approval UI. To require manual permission configuration instead, start it with `--auto-approve=false` or use the normal Swarmy permission flags and config.

## License

See [LICENSE](LICENSE).
