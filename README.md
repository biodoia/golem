# 🗿 GOLEM

**GLM-Powered CLI Coding Agent**

> The synthesis of Claude Code + OpenCode + Crush + Harmonica + Z.AI

<p align="center">
  <img src="./demo.gif" alt="Golem Demo" width="800">
</p>

```
   ██████╗  ██████╗ ██╗     ███████╗███╗   ███╗
  ██╔════╝ ██╔═══██╗██║     ██╔════╝████╗ ████║
  ██║  ███╗██║   ██║██║     █████╗  ██╔████╔██║
  ██║   ██║██║   ██║██║     ██╔══╝  ██║╚██╔╝██║
  ╚██████╔╝╚██████╔╝███████╗███████╗██║ ╚═╝ ██║
   ╚═════╝  ╚═════╝ ╚══════╝╚══════╝╚═╝     ╚═╝
```

## Features

### 🧠 Z.AI / GLM Integration (Native)
- **GLM-4-32B-0414** - Dialogue, engineering code, function calling
- **GLM-Z1-32B-0414** - Deep thinking, math, complex tasks
- **GLM-Z1-Rumination-32B** - Deep research, search-augmented reasoning
- **GLM-Z1-9B-0414** - Lightweight deployment, excellent efficiency
- **GLM-4.1V-9B-Thinking** - Vision + thinking capabilities
- OAuth authentication support
- Native MCP servers pre-configured

### 🎨 UI/UX
- **Crush-style graphics** - Beautiful terminal UI
- **Harmonica animations** - Physics-based spring animations
- **Bubbletea TUI** - Elm architecture for Go
- **Lipgloss styling** - Declarative terminal styling

### ⚡ Claude Code Features
- File operations (read, write, edit)
- Shell command execution
- MCP tool integration
- Session management
- Multi-turn conversations

### 🔧 OpenCode Features
- `/build` - Compile and test
- `/plan` - Multi-step planning
- `/agents` - Specialized agents (Architect, Coder, Reviewer)
- LSP integration
- Desktop app mode

### 🇨🇳 Chinese AI Ecosystem
- Zhipu AI (智谱AI) native support
- Baidu Wenxin (文心一言)
- Alibaba Tongyi Qianwen (通义千问)
- Tencent Hunyuan (腾讯混元)
- iFlytek Spark (讯飞星火)
- SiliconFlow aggregation

## Installation

```bash
go install github.com/biodoia/golem/cmd/golem@latest
```

Or build from source:
```bash
git clone https://github.com/biodoia/golem
cd golem
go build -o golem ./cmd/golem
```

## Quick Start

```bash
# Set your Z.AI API key
export ZHIPU_API_KEY=your_key_here

# Or use OAuth
golem auth login

# Start interactive session
golem

# One-shot query
golem "explain this code" --file main.go

# Use specific model
golem --model glm-z1-32b "solve this math problem"
```

## Configuration

```yaml
# ~/.golem/config.yaml
provider: zhipu
model: glm-4-32b-0414

# OAuth settings
auth:
  method: oauth  # or api_key
  client_id: your_client_id

# MCP servers (pre-configured)
mcp:
  servers:
    - name: filesystem
      command: npx @anthropic/mcp-server-filesystem
    - name: web-search
      command: npx @anthropic/mcp-server-web-search
    - name: zhipu-tools
      command: golem-mcp-zhipu

# UI settings
ui:
  theme: crush
  animations: harmonica
  fps: 60
```

## Commands

| Command | Description |
|---------|-------------|
| `/help` | Show help |
| `/model` | Switch model |
| `/build` | Build project |
| `/plan` | Create execution plan |
| `/agents` | Manage agents |
| `/mcp` | MCP server control |
| `/config` | Configuration |
| `/auth` | Authentication |

## Z.AI Models Reference

| Model | Parameters | Best For |
|-------|------------|----------|
| `glm-4-32b-0414` | 32B | Dialogue, code, function calling |
| `glm-z1-32b-0414` | 32B | Deep thinking, math, reasoning |
| `glm-z1-rumination-32b` | 32B | Research, complex open tasks |
| `glm-z1-9b-0414` | 9B | Lightweight, efficient |
| `glm-4.1v-9b-thinking` | 9B | Vision + reasoning |

## Architecture

```
┌────────────────────────────────────────────────┐
│                   GOLEM CLI                     │
├────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐     │
│  │  Crush   │  │Harmonica │  │ Bubbles  │     │
│  │   UI     │  │ Anims    │  │Components│     │
│  └──────────┘  └──────────┘  └──────────┘     │
├────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────┐  │
│  │            Agent Runtime                 │  │
│  │  ┌────────┐ ┌────────┐ ┌────────┐       │  │
│  │  │Architect│ │ Coder  │ │Reviewer│       │  │
│  │  └────────┘ └────────┘ └────────┘       │  │
│  └──────────────────────────────────────────┘  │
├────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────┐  │
│  │           MCP Integration                │  │
│  │  filesystem │ web │ zhipu-tools │ ...   │  │
│  └──────────────────────────────────────────┘  │
├────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────┐  │
│  │         Provider Layer                   │  │
│  │  Z.AI │ OpenAI │ Anthropic │ Local      │  │
│  └──────────────────────────────────────────┘  │
└────────────────────────────────────────────────┘
```

## License

MIT © Biodoia

---

*Named after the legendary Golem - brought to life through GLM 🗿*
