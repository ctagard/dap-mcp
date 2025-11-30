# DAP-MCP: Debug Adapter Protocol for AI Agents

An MCP (Model Context Protocol) server that gives AI agents the ability to debug code. Launch debug sessions, set breakpoints, inspect variables, and step through code - all through natural language.

## Why DAP-MCP?

Traditional debugging requires manually setting breakpoints, stepping through code, and inspecting state. DAP-MCP exposes these capabilities to LLMs, enabling:

- **AI-assisted debugging**: "Find why this function returns null"
- **Automated testing**: Set breakpoints and verify variable states
- **Code understanding**: Inspect runtime behavior to understand complex logic
- **Token-efficient**: Compound tools like `debug_snapshot` reduce API calls

## Supported Languages

| Language | Debug Adapter | Status |
|----------|--------------|--------|
| Go | [Delve](https://github.com/go-delve/delve) | Full support |
| Python | [debugpy](https://github.com/microsoft/debugpy) | Full support |
| JavaScript | [vscode-js-debug](https://github.com/microsoft/vscode-js-debug) | Full support |
| TypeScript | [vscode-js-debug](https://github.com/microsoft/vscode-js-debug) | Full support |
| React/Vue/Svelte | [vscode-js-debug](https://github.com/microsoft/vscode-js-debug) + Chrome | Full support |
| C | [LLDB](https://lldb.llvm.org/) / [GDB](https://www.gnu.org/software/gdb/) | Full support |
| C++ | [LLDB](https://lldb.llvm.org/) / [GDB](https://www.gnu.org/software/gdb/) | Full support |
| Rust | [LLDB](https://lldb.llvm.org/) / [GDB](https://www.gnu.org/software/gdb/) | Full support |
| Objective-C | [LLDB](https://lldb.llvm.org/) | Full support |
| Swift | [LLDB](https://lldb.llvm.org/) | Full support |

## Quick Start

### 1. Install DAP-MCP

**One-line install (recommended):**

```bash
curl -sSL https://raw.githubusercontent.com/ctagard/dap-mcp/main/scripts/install.sh | bash
```

This auto-detects your OS and installs using the best method:
- **macOS**: Homebrew (if available) or binary
- **Debian/Ubuntu**: .deb package
- **Fedora/RHEL**: .rpm package
- **Other Linux**: Binary

Options via environment variables:
```bash
# Install specific version
curl -sSL ... | INSTALL_VERSION=0.1.1 bash

# Install to custom directory
curl -sSL ... | INSTALL_DIR=$HOME/.local/bin bash

# Force binary install (skip package managers)
curl -sSL ... | INSTALL_METHOD=binary bash
```

<details>
<summary><strong>Other installation methods</strong></summary>

**Homebrew (macOS/Linux):**
```bash
brew install ctagard/tap/dap-mcp
```

**Debian/Ubuntu:**
```bash
curl -LO https://github.com/ctagard/dap-mcp/releases/latest/download/dap-mcp_0.1.1_linux_amd64.deb
sudo dpkg -i dap-mcp_0.1.1_linux_amd64.deb
```

**Fedora/RHEL/CentOS:**
```bash
curl -LO https://github.com/ctagard/dap-mcp/releases/latest/download/dap-mcp_0.1.1_linux_amd64.rpm
sudo rpm -i dap-mcp_0.1.1_linux_amd64.rpm
```

**Go Install:**
```bash
go install github.com/ctagard/dap-mcp/cmd/dap-mcp@latest
```
</details>

<details>
<summary><strong>Docker</strong></summary>

```bash
docker pull ghcr.io/ctagard/dap-mcp:latest

# Run with stdio (for MCP)
docker run -i ghcr.io/ctagard/dap-mcp:latest
```
</details>

<details>
<summary><strong>Manual Binary Download</strong></summary>

Download the appropriate archive from the [releases page](https://github.com/ctagard/dap-mcp/releases):

```bash
# macOS (Apple Silicon)
curl -L https://github.com/ctagard/dap-mcp/releases/download/v0.1.1/dap-mcp_0.1.1_darwin_arm64.tar.gz | tar xz
sudo mv dap-mcp /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/ctagard/dap-mcp/releases/download/v0.1.1/dap-mcp_0.1.1_darwin_amd64.tar.gz | tar xz
sudo mv dap-mcp /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/ctagard/dap-mcp/releases/download/v0.1.1/dap-mcp_0.1.1_linux_amd64.tar.gz | tar xz
sudo mv dap-mcp /usr/local/bin/

# Linux (arm64)
curl -L https://github.com/ctagard/dap-mcp/releases/download/v0.1.1/dap-mcp_0.1.1_linux_arm64.tar.gz | tar xz
sudo mv dap-mcp /usr/local/bin/

# Windows (amd64)
# Download dap-mcp_0.1.1_windows_amd64.zip from releases page
```
</details>

<details>
<summary><strong>Build from Source</strong></summary>

```bash
git clone https://github.com/ctagard/dap-mcp
cd dap-mcp
make build
# Binary is at ./bin/dap-mcp
```
</details>

### 2. Install Debug Adapters

Install the adapter(s) for languages you want to debug:

```bash
# Go - Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Python - debugpy
pip install debugpy

# JavaScript/TypeScript - vscode-js-debug (see detailed instructions below)

# C/C++/Rust/Objective-C/Swift - LLDB (usually pre-installed on macOS)
# macOS: Included with Xcode Command Line Tools
xcode-select --install

# Linux (Ubuntu/Debian)
sudo apt install lldb

# Linux (Fedora)
sudo dnf install lldb

# Or use GDB instead
sudo apt install gdb   # Debian/Ubuntu
sudo dnf install gdb   # Fedora
```

### 3. Configure Your AI Client

<details>
<summary><strong>Claude Code</strong></summary>

Add to `~/.claude.json`:

```json
{
  "mcpServers": {
    "dap-mcp": {
      "command": "dap-mcp",
      "args": ["--mode", "full"]
    }
  }
}
```

Or with a config file:
```json
{
  "mcpServers": {
    "dap-mcp": {
      "command": "dap-mcp",
      "args": ["--config", "/path/to/dap-mcp-config.json"]
    }
  }
}
```

</details>

<details>
<summary><strong>Claude Desktop</strong></summary>

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "dap-mcp": {
      "command": "/usr/local/bin/dap-mcp",
      "args": ["--mode", "full"]
    }
  }
}
```

</details>

<details>
<summary><strong>Cursor</strong></summary>

Add to your settings (Cmd/Ctrl + Shift + P → "Preferences: Open User Settings (JSON)"):

```json
{
  "mcp.servers": {
    "dap-mcp": {
      "command": "/usr/local/bin/dap-mcp",
      "args": ["--mode", "full"]
    }
  }
}
```

</details>

<details>
<summary><strong>Continue.dev</strong></summary>

Add to `~/.continue/config.json`:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "stdio",
          "command": "/usr/local/bin/dap-mcp",
          "args": ["--mode", "full"]
        }
      }
    ]
  }
}
```

</details>

<details>
<summary><strong>Other MCP Clients</strong></summary>

DAP-MCP uses stdio transport. Configure your client with:
- **Command**: Path to `dap-mcp` binary
- **Arguments**: `["--mode", "full"]` or `["--config", "/path/to/config.json"]`
- **Transport**: stdio

</details>

### 4. Start Debugging!

Ask your AI assistant:

> "Debug my Python script at /path/to/script.py. Set a breakpoint at line 42 and show me the variables when it stops there."

> "Launch a debug session for my Go program and step through the main function."

> "Attach to my React app running at localhost:3000 and debug the button click handler."

## Configuration

### Command Line Options

```
dap-mcp [OPTIONS]

OPTIONS:
    --config <path>   Path to configuration file (JSON)
    --mode <mode>     Capability mode: 'readonly' or 'full' (default: full)
    --version         Show version and exit
    --help            Show help message
```

### Configuration File

Create a JSON file for advanced settings:

```json
{
  "mode": "full",
  "allowSpawn": true,
  "allowAttach": true,
  "allowModify": true,
  "allowExecute": true,
  "maxSessions": 10,
  "adapters": {
    "go": {
      "path": "dlv",
      "buildFlags": "-gcflags='all=-N -l'"
    },
    "python": {
      "pythonPath": "python3"
    },
    "node": {
      "nodePath": "node",
      "jsDebugPath": "/path/to/vscode-js-debug/src/dapDebugServer.js",
      "sourceMapPathOverrides": {
        "/*": "${webRoot}/*",
        "webpack:///src/*": "${webRoot}/src/*"
      }
    }
  }
}
```

### Security Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `full` | All debugging capabilities | Development, trusted environments |
| `readonly` | Inspect only, no execution control | Production monitoring, untrusted code |

Fine-grained permissions:
- `allowSpawn`: Can start new debug processes
- `allowAttach`: Can attach to running processes
- `allowModify`: Can modify variable values
- `allowExecute`: Can evaluate arbitrary expressions

## Available Tools

DAP-MCP provides a streamlined 12-tool API designed for LLM efficiency.

### Session Management (4 tools)

| Tool | Description |
|------|-------------|
| `debug_launch` | Launch a new debug session. Supports direct args OR VS Code launch.json configs |
| `debug_attach` | Attach to a running process or browser |
| `debug_disconnect` | End a debug session |
| `debug_list_sessions` | List all active debug sessions |

### Inspection (2 tools - available in all modes)

| Tool | Description |
|------|-------------|
| `debug_snapshot` | **Primary inspection tool** - Get complete state (threads, stack, scopes, variables) in ONE call |
| `debug_evaluate` | Evaluate single or multiple expressions. Supports batch mode with `expressions` JSON array |

### Control (6 tools - full mode only)

| Tool | Description |
|------|-------------|
| `debug_breakpoints` | Set breakpoints in a source file (replaces all breakpoints in file) |
| `debug_step` | Step with `type`: 'over' (next line), 'into' (enter function), 'out' (exit function) |
| `debug_continue` | Continue execution until next breakpoint |
| `debug_pause` | Pause program execution |
| `debug_set_variable` | Modify a variable's value |
| `debug_run_to_line` | Run to specific line and return snapshot (combines breakpoint + continue + snapshot) |

## Language-Specific Setup

### Go

Delve is the standard Go debugger. Install it:

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

Verify installation:
```bash
dlv version
```

### Python

debugpy is Microsoft's debug adapter for Python:

```bash
pip install debugpy
```

For virtual environments, ensure debugpy is installed in the environment you're debugging.

### JavaScript/TypeScript (Node.js)

vscode-js-debug is required for JavaScript/TypeScript debugging:

**Option 1: Download pre-built**
```bash
# Download from releases
curl -L https://github.com/anthropics/dap-mcp/releases/latest/download/js-debug.tar.gz -O
tar xzf js-debug.tar.gz -C ~/.local/share/
```

**Option 2: Build from source**
```bash
git clone https://github.com/microsoft/vscode-js-debug
cd vscode-js-debug
npm install --legacy-peer-deps
npx gulp vsDebugServerBundle
mv dist src  # The server expects files in src/
```

Configure the path in your dap-mcp config:
```json
{
  "adapters": {
    "node": {
      "jsDebugPath": "/path/to/vscode-js-debug/src/dapDebugServer.js"
    }
  }
}
```

### C/C++/Rust (LLDB)

LLDB is the recommended debugger for C, C++, Rust, Objective-C, and Swift:

```bash
# macOS - included with Xcode Command Line Tools
xcode-select --install

# Verify installation
lldb --version
```

For Linux, install LLDB:
```bash
# Ubuntu/Debian
sudo apt install lldb

# Fedora
sudo dnf install lldb
```

Compile your code with debug symbols:
```bash
# C/C++
gcc -g -O0 myprogram.c -o myprogram
g++ -g -O0 myprogram.cpp -o myprogram
clang -g -O0 myprogram.c -o myprogram

# Rust
cargo build  # Debug build includes symbols by default
```

### C/C++ (GDB)

GDB is an alternative debugger, especially on Linux:

```bash
# Ubuntu/Debian
sudo apt install gdb

# Fedora
sudo dnf install gdb

# Verify installation
gdb --version
```

### React/Vue/Svelte (Browser Debugging)

For frontend frameworks, you debug through Chrome:

1. Start your dev server: `npm run dev`
2. Use `target: "chrome"` with the URL:

```json
{
  "tool": "debug_launch",
  "args": {
    "language": "javascript",
    "target": "chrome",
    "program": "http://localhost:3000",
    "webRoot": "/path/to/your/project"
  }
}
```

For source map resolution with bundlers (Vite, Webpack, etc.), configure `sourceMapPathOverrides`:

```json
{
  "adapters": {
    "node": {
      "jsDebugPath": "/path/to/dapDebugServer.js",
      "sourceMapPathOverrides": {
        "/*": "${webRoot}/*",
        "webpack:///src/*": "${webRoot}/src/*",
        "webpack:///./*": "${webRoot}/*"
      }
    }
  }
}
```

## Example Workflows

### Debug a Go Program

```
User: Debug main.go and stop at line 42

AI uses:
1. debug_launch(language="go", program="./main.go", stopOnEntry=true)
2. debug_run_to_line(path="main.go", line=42)  → Returns snapshot with stack and variables
```

### Debug a Python Script

```
User: Why does process_data return None for this input?

AI uses:
1. debug_launch(language="python", program="script.py", stopOnEntry=true)
2. debug_run_to_line(path="script.py", line=45)  → Runs to return statement
3. debug_evaluate(expressions='["result", "len(data)", "type(data)"]')  → Batch evaluate
4. Analyzes variable state to explain the bug
```

### Debug a React App

```
User: Debug the click handler in App.jsx

AI uses:
1. debug_launch(language="javascript", target="chrome",
                program="http://localhost:3000", webRoot="/path/to/project")
2. debug_breakpoints(path="/path/to/project/src/App.jsx",
                     breakpoints='[{"line": 25}]')
3. [User clicks button in browser]
4. debug_snapshot() → Returns component state, props at breakpoint
```

### Use VS Code launch.json Configuration

```
User: Debug my project using the "Python: Tests" configuration

AI uses:
1. debug_launch(configName="Python: Tests", workspace="/path/to/project")
   → Loads settings from .vscode/launch.json, resolves ${workspaceFolder}, etc.
2. debug_snapshot() → Returns state at entry point
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Claude / Cursor / AI Client                  │
│                           (MCP Client)                           │
└─────────────────────┬───────────────────────────────────────────┘
                      │ MCP Protocol (JSON-RPC 2.0 over stdio)
┌─────────────────────▼───────────────────────────────────────────┐
│                       DAP-MCP Server                             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  MCP Handler → Session Manager → DAP Client             │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────┬───────────────────────────────────────────┘
                      │ DAP Protocol (JSON over TCP)
┌─────────────────────▼───────────────────────────────────────────┐
│  ┌──────────┐  ┌──────────┐  ┌─────────────────────────────┐    │
│  │   Delve  │  │ debugpy  │  │      vscode-js-debug        │    │
│  │   (Go)   │  │ (Python) │  │   (JavaScript/TypeScript)   │    │
│  └──────────┘  └──────────┘  └──────────────┬──────────────┘    │
└──────────────────────────────────────────────│──────────────────┘
                                               │ CDP
                                     ┌─────────▼─────────┐
                                     │  Node.js / Chrome │
                                     └───────────────────┘
```

## Troubleshooting

### "dlv not found" / "debugpy not found"

Ensure the debug adapter is in your PATH. For Go:
```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### "jsDebugPath not configured"

You need to install vscode-js-debug and configure its path. See [JavaScript/TypeScript setup](#javascripttypescript-nodejs).

### Breakpoints show "Unbound"

For browser debugging, this usually means source maps aren't resolving correctly. Try:
1. Ensure `webRoot` points to your project root
2. Configure `sourceMapPathOverrides` for your bundler
3. Check that source maps are enabled in your bundler config

### "Connection refused" on attach

Ensure the target process is:
1. Running with debug flags (`--inspect` for Node, `--remote-debugging-port` for Chrome)
2. Listening on the expected port

### Session times out

Increase the timeout in your config:
```json
{
  "sessionTimeout": "60m"
}
```

## Development

```bash
# Build
make build

# Run tests
make test

# Run linter
make lint

# Build for all platforms
make build-all
```

## Related Projects

- [Model Context Protocol](https://modelcontextprotocol.io/) - The protocol this server implements
- [Debug Adapter Protocol](https://microsoft.github.io/debug-adapter-protocol/) - The debugging protocol used internally
- [Delve](https://github.com/go-delve/delve) - Go debugger
- [debugpy](https://github.com/microsoft/debugpy) - Python debug adapter
- [vscode-js-debug](https://github.com/microsoft/vscode-js-debug) - JavaScript debug adapter

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
