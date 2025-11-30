# typed: false
# frozen_string_literal: true

# Homebrew formula for dap-mcp
# To install locally for testing: brew install --build-from-source ./Formula/dap-mcp.rb
class DapMcp < Formula
  desc "MCP server exposing Debug Adapter Protocol to AI agents"
  homepage "https://github.com/ctagard/dap-mcp"
  license "MIT"
  head "https://github.com/ctagard/dap-mcp.git", branch: "main"

  # Stable release (update these for each release)
  # url "https://github.com/ctagard/dap-mcp/archive/refs/tags/v0.1.0.tar.gz"
  # sha256 "PLACEHOLDER"
  # version "0.1.0"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X main.version=#{version}
    ]
    system "go", "build", *std_go_args(ldflags:), "./cmd/dap-mcp"
  end

  def caveats
    <<~EOS
      To use dap-mcp with Claude Code, add to ~/.claude.json:
        {
          "mcpServers": {
            "dap-mcp": {
              "command": "#{bin}/dap-mcp",
              "args": ["--mode", "full"]
            }
          }
        }

      For debugging support, install the appropriate debug adapters:
        Go:     go install github.com/go-delve/delve/cmd/dlv@latest
        Python: pip install debugpy
        JS/TS:  See https://github.com/ctagard/dap-mcp#javascripttypescript-nodejs
    EOS
  end

  test do
    # Test that the binary runs and shows version
    assert_match "dap-mcp", shell_output("#{bin}/dap-mcp --help 2>&1")
  end
end
