# Zen Claw Gateway API Documentation

The Zen Claw Gateway provides a REST API for AI agent functionality. All endpoints return JSON.

## Base URL
```
http://localhost:8080
```

## Authentication
Currently no authentication is required (local development only).

## Endpoints

### Health Check
Check if the gateway is running.

**Endpoint:** `GET /health`

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2026-02-02T09:34:08-05:00",
  "gateway": "zen-claw",
  "version": "0.1.0"
}
```

### Chat
Send a chat request to the AI agent.

**Endpoint:** `POST /chat`

**Request Body:**
```json
{
  "session_id": "string (optional)",
  "user_input": "string (required)",
  "working_dir": "string (optional, default: '.')",
  "provider": "string (optional, default: 'deepseek')",
  "model": "string (optional, default: provider's default model)",
  "max_steps": "integer (optional, default: 20)"
}
```

**Field Descriptions:**
- `session_id`: Unique identifier for the session. If not provided, a new session ID is generated.
- `user_input`: The task or question for the AI agent.
- `working_dir`: Working directory for tool execution (file operations, commands).
- `provider`: AI provider to use: `deepseek`, `qwen`, `glm`, `minimax`, or `openai`.
- `model`: Specific model to use with the provider.
- `max_steps`: Maximum number of tool execution steps before stopping.

**Response:**
```json
{
  "session_id": "session_20260202_093425",
  "result": "The AI agent's response",
  "session_info": {
    "session_id": "session_20260202_093425",
    "created_at": "2026-02-02T09:34:25-05:00",
    "updated_at": "2026-02-02T09:34:27-05:00",
    "message_count": 3,
    "user_messages": 1,
    "assistant_messages": 1,
    "tool_messages": 0,
    "working_dir": "."
  },
  "error": "string (optional, present if error occurred)"
}
```

**Example Request:**
```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{
    "user_input": "What is in the current directory?",
    "session_id": "test-session",
    "provider": "qwen",
    "model": "qwen3-coder-30b-a3b-instruct",
    "working_dir": ".",
    "max_steps": 10
  }'
```

### List Sessions
Get all active sessions.

**Endpoint:** `GET /sessions`

**Response:**
```json
{
  "sessions": [
    {
      "id": "session_20260202_093425",
      "created_at": "2026-02-02T09:34:25-05:00",
      "updated_at": "2026-02-02T09:34:27-05:00",
      "message_count": 3,
      "user_messages": 1,
      "assistant_messages": 1,
      "tool_messages": 0,
      "working_dir": "."
    }
  ],
  "count": 1
}
```

### Get Session
Get details of a specific session.

**Endpoint:** `GET /sessions/{session_id}`

**Response:**
```json
{
  "id": "session_20260202_093425",
  "created_at": "2026-02-02T09:34:25-05:00",
  "updated_at": "2026-02-02T09:34:27-05:00",
  "message_count": 3,
  "user_messages": 1,
  "assistant_messages": 1,
  "tool_messages": 0,
  "working_dir": ".",
  "messages": [
    {
      "role": "system",
      "content": "You are a strategic AI assistant..."
    },
    {
      "role": "user",
      "content": "What is in the current directory?"
    },
    {
      "role": "assistant",
      "content": "I'll check the current directory for you..."
    }
  ]
}
```

### Delete Session
Delete a specific session.

**Endpoint:** `DELETE /sessions/{session_id}`

**Response:**
```json
{
  "deleted": true,
  "id": "session_20260202_093425"
}
```

## Available AI Providers

### DeepSeek
- **Provider name:** `deepseek`
- **Default model:** `deepseek-chat`
- **Base URL:** `https://api.deepseek.com`
- **Models:**
  - `deepseek-chat`
  - `deepseek-reasoner`

### Qwen (Alibaba Cloud)
- **Provider name:** `qwen`
- **Default model:** `qwen3-coder-30b-a3b-instruct`
- **Base URL:** `https://dashscope-us.aliyuncs.com/compatible-mode/v1`
- **Models:**
  - `qwen3-coder-30b-a3b-instruct` (262K context)
  - `qwen-plus`
  - `qwen-max`
  - `qwen3-235b-a22b-instruct-2507`
  - `qwen3-coder-480b-a35b-instruct`

### GLM (Zhipu AI)
- **Provider name:** `glm`
- **Default model:** `glm-4.7`
- **Base URL:** `https://open.bigmodel.cn/api/paas/v4`
- **Models:**
  - `glm-4.7`
  - `glm-4`
  - `glm-3-turbo`

### Minimax
- **Provider name:** `minimax`
- **Default model:** `minimax-M2.1`
- **Base URL:** `https://api.minimax.chat/v1`
- **Models:**
  - `minimax-M2.1`
  - `abab6.5s`
  - `abab6.5`

### OpenAI
- **Provider name:** `openai`
- **Default model:** `gpt-4o-mini`
- **Base URL:** `https://api.openai.com/v1`
- **Models:**
  - `gpt-4o-mini`
  - `gpt-4o`
  - `gpt-4-turbo`
  - `gpt-3.5-turbo`

## Available Tools

The AI agent has access to these tools:

### exec
Execute shell commands.

**Parameters:**
```json
{
  "command": "string (required)"
}
```

**Example:** `{"command": "ls -la"}`

### read_file
Read file contents.

**Parameters:**
```json
{
  "path": "string (required)"
}
```

**Example:** `{"path": "README.md"}`

### list_dir
List directory contents.

**Parameters:**
```json
{
  "path": "string (optional, default: '.')"
}
```

**Example:** `{"path": "src/"}`

### system_info
Get system information.

**Parameters:** `{}` (no parameters)

## Session Persistence

Sessions are persisted to disk in `/tmp/zen-claw-sessions/`. Each session is stored as a JSON file named `{session_id}.json`.

**Session file format:**
```json
{
  "id": "session_20260202_093425",
  "created_at": "2026-02-02T09:34:25-05:00",
  "updated_at": "2026-02-02T09:34:27-05:00",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ],
  "working_dir": "."
}
```

## Error Responses

### Common HTTP Status Codes
- `200 OK`: Request successful
- `400 Bad Request`: Invalid request parameters
- `404 Not Found`: Session not found
- `405 Method Not Allowed`: Wrong HTTP method
- `500 Internal Server Error`: Server error

### Error Response Format
```json
{
  "error": "Error message describing what went wrong"
}
```

## Rate Limiting

Currently no rate limiting is implemented (local development only).

## WebSocket Support

Not yet implemented. Future versions may include WebSocket for real-time streaming.

## Client Libraries

### Go Client Example
```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type ChatRequest struct {
    SessionID  string `json:"session_id,omitempty"`
    UserInput  string `json:"user_input"`
    WorkingDir string `json:"working_dir,omitempty"`
    Provider   string `json:"provider,omitempty"`
    Model      string `json:"model,omitempty"`
    MaxSteps   int    `json:"max_steps,omitempty"`
}

type ChatResponse struct {
    SessionID   string                 `json:"session_id"`
    Result      string                 `json:"result"`
    SessionInfo map[string]interface{} `json:"session_info"`
    Error       string                 `json:"error,omitempty"`
}

func main() {
    req := ChatRequest{
        UserInput: "What is in the current directory?",
        Provider:  "qwen",
        Model:     "qwen3-coder-30b-a3b-instruct",
    }
    
    jsonData, _ := json.Marshal(req)
    resp, err := http.Post("http://localhost:8080/chat", 
        "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    
    var result ChatResponse
    json.NewDecoder(resp.Body).Decode(&result)
    
    fmt.Println("Result:", result.Result)
    fmt.Println("Session ID:", result.SessionID)
}
```

### Python Client Example
```python
import requests
import json

url = "http://localhost:8080/chat"
payload = {
    "user_input": "What is in the current directory?",
    "provider": "qwen",
    "model": "qwen3-coder-30b-a3b-instruct"
}

response = requests.post(url, json=payload)
data = response.json()

print(f"Result: {data['result']}")
print(f"Session ID: {data['session_id']}")
```

## Configuration

The gateway reads configuration from:
1. Environment variables
2. Config file: `~/.zen/zen-claw/config.yaml`

**Environment variables:**
```bash
export DEEPSEEK_API_KEY="sk-..."
export QWEN_API_KEY="sk-..."
export GLM_API_KEY="sk-..."
export MINIMAX_API_KEY="sk-..."
export OPENAI_API_KEY="sk-..."
```

## Logging

Gateway logs are written to `/tmp/zen-gateway-*.log`. Enable verbose logging with the `--verbose` flag when starting the gateway.

## Monitoring

Check gateway health:
```bash
curl http://localhost:8080/health
```

Check active sessions:
```bash
curl http://localhost:8080/sessions
```

## Troubleshooting API Issues

1. **Gateway not responding**: Check if gateway is running with `curl http://localhost:8080/health`
2. **Invalid API key**: Check API keys in config or environment variables
3. **Session not found**: Verify session ID exists with `GET /sessions`
4. **Provider not available**: Check if provider is configured and API key is valid
5. **Model not found**: Verify model name is correct for the provider