# Building and Running Zen Claw

## Go 1.24 (Compatible with ~/zen projects)

```bash
# Check Go version (1.24 for compatibility)
go version

# Install dependencies
go mod tidy

# Build binary
go build -o zen-claw .

# Test CLI
./zen-claw --help
./zen-claw agent --help
./zen-claw tools
```

## ✅ Verified Build

The project has been successfully built and tested with:
- **Go 1.24.3** (system default at `/usr/local/go/bin/go`, compatible with ~/zen)
- **Cobra CLI framework** v1.8.1
- **Binary**: `zen-claw` (13MB with OpenAI SDK, Linux amd64)
- **Compatibility**: Safe with existing ~/zen projects

## Development

```bash
# Run tests (when written)
go test ./...

# Format code
go fmt ./...

# Check imports
go mod tidy

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o zen-claw-linux .
GOOS=darwin GOARCH=arm64 go build -o zen-claw-macos .
```

## Dependencies

Current dependencies (see `go.mod`):
- `github.com/spf13/cobra` - CLI framework

To add new dependencies:
```bash
go get github.com/some/new-package
go mod tidy
```

## Project Structure for Go Developers

```
zen-claw/
├── main.go                 # Entry point
├── cmd/                    # CLI commands (cobra)
│   ├── root.go            # Root command
│   ├── agent.go           # Agent command
│   ├── session.go         # Session command
│   ├── tools.go           # Tools command
│   └── gateway.go         # Gateway command
├── internal/              # Private packages
│   ├── agent/            # Agent core
│   │   ├── agent.go      # Main agent
│   │   └── ai_agent.go   # AI integration
│   ├── session/          # Session management
│   ├── tools/            # Tool implementations
│   └── ai/               # AI provider interface
├── go.mod                # Go module
├── go.sum               # Dependency checksums
└── README.md            # Documentation
```

## Adding a New Tool

1. Create tool in `internal/tools/`:
```go
package tools

type NewTool struct{}

func (t *NewTool) Name() string { return "newtool" }
func (t *NewTool) Description() string { return "Does something new" }
func (t *NewTool) Execute(args map[string]interface{}) (interface{}, error) {
    // Implementation
}
```

2. Register in `manager.go`:
```go
func (m *Manager) registerCoreTools() {
    // ...
    m.tools["newtool"] = &NewTool{}
}
```

## Adding a New AI Provider

1. Implement `ai.Provider` interface:
```go
package providers

type OpenAIProvider struct {
    apiKey string
}

func (p *OpenAIProvider) Name() string { return "openai" }
func (p *OpenAIProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
    // Call OpenAI API
}
```

2. Register in agent initialization.

## Docker (Optional)

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o zen-claw .

FROM alpine:latest
COPY --from=builder /app/zen-claw /usr/local/bin/
ENTRYPOINT ["zen-claw"]
```

## Release Checklist

1. ✅ All code formatted (`go fmt`)
2. ✅ Dependencies updated (`go mod tidy`)
3. ✅ Tests passing (`go test`)
4. ✅ Documentation updated
5. ✅ Binary builds (`go build`)
6. ✅ Commit to `main`
7. ✅ Tag version (`git tag v0.1.0`)

## Troubleshooting

**"command not found: go"**
- Install Go: https://go.dev/dl/
- Add to PATH: `export PATH=$PATH:/usr/local/go/bin`

**"missing go.sum entry"**
- Run: `go mod tidy`

**"import cycle not allowed"**
- Restructure packages to avoid circular dependencies
- Use interfaces to break cycles

**"tool not found"**
- Check tool is registered in `manager.go`
- Tool name must match exactly