package mcp

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/neves/zen-claw/internal/agent"
	"github.com/neves/zen-claw/internal/ai"
)

// ServerConfig defines an MCP server connection
type ServerConfig struct {
	Name    string   // Display name for the server
	Command string   // Command to run (e.g., "npx", "python", "go")
	Args    []string // Arguments to the command
	Env     []string // Environment variables (optional)
}

// Client manages connections to MCP servers
type Client struct {
	mu      sync.RWMutex
	servers map[string]*serverConn
}

type serverConn struct {
	config    ServerConfig
	transport *transport.Stdio
	client    *client.Client
	tools     []mcp.Tool
}

// NewClient creates a new MCP client manager
func NewClient() *Client {
	return &Client{
		servers: make(map[string]*serverConn),
	}
}

// Connect connects to an MCP server
func (c *Client) Connect(ctx context.Context, cfg ServerConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already connected
	if _, exists := c.servers[cfg.Name]; exists {
		return fmt.Errorf("server %s already connected", cfg.Name)
	}

	log.Printf("[MCP] Connecting to server: %s (%s %v)", cfg.Name, cfg.Command, cfg.Args)

	// Create stdio transport
	stdio := transport.NewStdio(cfg.Command, cfg.Env, cfg.Args...)
	if err := stdio.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MCP transport: %w", err)
	}

	// Create client with transport
	mcpClient := client.NewClient(stdio)

	// Initialize connection
	initReq := mcp.InitializeRequest{}
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "zen-claw",
		Version: "1.0.0",
	}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION

	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		stdio.Close()
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	// List available tools
	toolsResp, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		stdio.Close()
		return fmt.Errorf("failed to list MCP tools: %w", err)
	}

	log.Printf("[MCP] Server %s connected with %d tools", cfg.Name, len(toolsResp.Tools))
	for _, tool := range toolsResp.Tools {
		log.Printf("[MCP]   - %s: %s", tool.Name, tool.Description)
	}

	c.servers[cfg.Name] = &serverConn{
		config:    cfg,
		transport: stdio,
		client:    mcpClient,
		tools:     toolsResp.Tools,
	}

	return nil
}

// Disconnect disconnects from an MCP server
func (c *Client) Disconnect(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, exists := c.servers[name]
	if !exists {
		return fmt.Errorf("server %s not connected", name)
	}

	if err := conn.transport.Close(); err != nil {
		log.Printf("[MCP] Warning: error closing connection to %s: %v", name, err)
	}

	delete(c.servers, name)
	log.Printf("[MCP] Disconnected from server: %s", name)
	return nil
}

// Close closes all MCP connections
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, conn := range c.servers {
		if err := conn.transport.Close(); err != nil {
			log.Printf("[MCP] Warning: error closing %s: %v", name, err)
		}
	}
	c.servers = make(map[string]*serverConn)
}

// ListServers returns connected server names
func (c *Client) ListServers() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.servers))
	for name := range c.servers {
		names = append(names, name)
	}
	return names
}

// GetTools returns all tools from all connected servers as zen-claw tools
func (c *Client) GetTools() []agent.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var tools []agent.Tool
	for serverName, conn := range c.servers {
		for _, mcpTool := range conn.tools {
			tools = append(tools, &MCPTool{
				serverName: serverName,
				mcpTool:    mcpTool,
				client:     c,
			})
		}
	}
	return tools
}

// CallTool calls a tool on an MCP server
func (c *Client) CallTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (string, error) {
	c.mu.RLock()
	conn, exists := c.servers[serverName]
	c.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("server %s not connected", serverName)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args

	result, err := conn.client.CallTool(ctx, req)
	if err != nil {
		return "", fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Extract text content from result
	var content string
	for _, c := range result.Content {
		if textContent, ok := c.(mcp.TextContent); ok {
			content += textContent.Text
		}
	}

	if result.IsError {
		return "", fmt.Errorf("MCP tool error: %s", content)
	}

	return content, nil
}

// MCPTool wraps an MCP tool as a zen-claw agent.Tool
type MCPTool struct {
	serverName string
	mcpTool    mcp.Tool
	client     *Client
}

func (t *MCPTool) Name() string {
	return fmt.Sprintf("mcp_%s_%s", t.serverName, t.mcpTool.Name)
}

func (t *MCPTool) Description() string {
	return fmt.Sprintf("[MCP:%s] %s", t.serverName, t.mcpTool.Description)
}

func (t *MCPTool) Parameters() map[string]interface{} {
	// Convert MCP tool input schema to our format
	if t.mcpTool.InputSchema.Properties == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	return map[string]interface{}{
		"type":       "object",
		"properties": t.mcpTool.InputSchema.Properties,
		"required":   t.mcpTool.InputSchema.Required,
	}
}

func (t *MCPTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	result, err := t.client.CallTool(ctx, t.serverName, t.mcpTool.Name, args)
	if err != nil {
		return map[string]interface{}{
			"error":   err.Error(),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"result":  result,
		"success": true,
	}, nil
}

// GetToolDefinitions returns AI-compatible tool definitions for all MCP tools
func (c *Client) GetToolDefinitions() []ai.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var tools []ai.Tool
	for serverName, conn := range c.servers {
		for _, mcpTool := range conn.tools {
			tools = append(tools, ai.Tool{
				Name:        fmt.Sprintf("mcp_%s_%s", serverName, mcpTool.Name),
				Description: fmt.Sprintf("[MCP:%s] %s", serverName, mcpTool.Description),
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": mcpTool.InputSchema.Properties,
					"required":   mcpTool.InputSchema.Required,
				},
			})
		}
	}
	return tools
}
