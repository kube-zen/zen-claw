# Zen Claw Gateway API Documentation

The Zen Claw Gateway provides a REST API for AI agent functionality with real-time SSE streaming.

## Base URL
```
http://localhost:8080
```

## Authentication
Currently no authentication required (local development only).

## Endpoints

### Health Check
Check if the gateway is running.

**Endpoint:** `GET /health`

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2026-02-03T05:42:03-05:00",
  "gateway": "zen-claw",
  "version": "0.1.0"
}
```

---

### Chat (Blocking)
Send a chat request and wait for completion.

**Endpoint:** `POST /chat`

**Request Body:**
```json
{
  "session_id": "string (optional)",
  "user_input": "string (required)",
  "working_dir": "string (optional, default: '.')",
  "provider": "string (optional, default: 'deepseek')",
  "model": "string (optional, default: provider's default)",
  "max_steps": "integer (optional, default: 100)"
}
```

**Response:**
```json
{
  "session_id": "session_20260203_054213",
  "result": "The AI agent's response",
  "session_info": {
    "session_id": "session_20260203_054213",
    "created_at": "2026-02-03T05:42:13-05:00",
    "updated_at": "2026-02-03T05:42:15-05:00",
    "message_count": 3,
    "user_messages": 1,
    "assistant_messages": 1,
    "tool_messages": 0,
    "working_dir": "."
  },
  "error": "string (optional)"
}
```

---

### Chat with Streaming (SSE)
Send a chat request with real-time progress events via Server-Sent Events.

**Endpoint:** `POST /chat/stream`

**Request Body:** Same as `/chat`

**Headers:**
```
Content-Type: application/json
Accept: text/event-stream
```

**Response:** Server-Sent Events stream

**Event Types:**

| Type | Description | Data Fields |
|------|-------------|-------------|
| `start` | Agent started | `provider`, `model`, `message` |
| `step` | New step started | `step`, `message` |
| `thinking` | Waiting for AI | `step`, `message` |
| `ai_response` | AI reasoning text | `step`, `message` |
| `tool_call` | Tool being executed | `step`, `message`, `data.tool`, `data.args` |
| `tool_result` | Tool completed | `step`, `message` |
| `complete` | Task finished | `step`, `message`, `data.total_steps` |
| `error` | Error occurred | `message` |
| `done` | Final result | `session_id`, `result`, `session_info` |

**Example Event Stream:**
```
data: {"type":"start","provider":"deepseek","model":"deepseek-chat","message":"Starting with deepseek/deepseek-chat"}

data: {"type":"step","step":1,"message":"Step 1/100: Thinking..."}

data: {"type":"thinking","step":1,"message":"Waiting for AI response..."}

data: {"type":"tool_call","step":1,"message":"🔧 list_dir(path=\".\")","data":{"tool":"list_dir","args":{"path":"."}}}

data: {"type":"tool_result","step":1,"message":"✓ list_dir → 34 items"}

data: {"type":"complete","step":1,"message":"Task completed","data":{"total_steps":1}}

data: {"type":"done","session_id":"session_123","result":"Here are the files...","session_info":{...}}
```

**Example Usage (curl):**
```bash
curl -N -X POST http://localhost:8080/chat/stream \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"user_input": "list files", "working_dir": "."}'
```

---

### WebSocket
Bidirectional communication for real-time interaction.

**Endpoint:** `GET /ws` (WebSocket upgrade)

**Connection:**
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
```

**Message Format:**
```json
{
  "type": "string",      // Message type
  "id": "string",        // Request ID for matching responses
  "data": {}             // Payload (type-specific)
}
```

**Client → Server Messages:**

| Type | Description | Data Fields |
|------|-------------|-------------|
| `chat` | Send chat request | `session_id`, `user_input`, `working_dir`, `provider`, `model`, `max_steps` |
| `cancel` | Cancel current task | (none) |
| `ping` | Keep-alive ping | (none) |
| `sessions` | List sessions | (none) |
| `session` | Get/delete session | `session_id`, `action` ("get" or "delete") |

**Server → Client Messages:**

| Type | Description | Data Fields |
|------|-------------|-------------|
| `connected` | Connection established | `message`, `version` |
| `progress` | Task progress event | `type`, `step`, `message`, `data` |
| `result` | Task completed | `session_id`, `result`, `session_info` |
| `error` | Error occurred | `error` |
| `cancelled` | Task cancelled | `message` |
| `pong` | Ping response | (none) |
| `sessions` | Session list | `sessions`, `count` |
| `session` | Session details | (session stats) |

**Example Chat Flow:**
```json
// Client sends
{"type": "chat", "id": "msg_1", "data": {"user_input": "hello", "provider": "deepseek"}}

// Server sends progress events
{"type": "progress", "id": "msg_1", "data": {"type": "start", "message": "Starting..."}}
{"type": "progress", "id": "msg_1", "data": {"type": "step", "step": 1, "message": "Thinking..."}}

// Server sends result
{"type": "result", "id": "msg_1", "data": {"session_id": "...", "result": "Hello!", "session_info": {...}}}
```

**Example Cancel:**
```json
// Client sends
{"type": "cancel"}

// Server responds
{"type": "cancelled", "id": "msg_1", "data": {"message": "Task cancelled"}}
```

**CLI Usage:**
```bash
# Use WebSocket instead of SSE
./zen-claw agent --ws "analyze codebase"
```

---

### List Sessions
Get all sessions with their state.

**Endpoint:** `GET /sessions`

**Response:**
```json
{
  "sessions": [
    {
      "id": "session_20260203_054213",
      "created_at": "2026-02-03T05:42:13-05:00",
      "updated_at": "2026-02-03T05:42:15-05:00",
      "message_count": 3,
      "user_messages": 1,
      "assistant_messages": 1,
      "tool_messages": 0,
      "working_dir": ".",
      "state": "active",
      "client_id": "",
      "last_used": "2026-02-03T05:42:15-05:00"
    }
  ],
  "count": 1,
  "max_sessions": 5,
  "active_count": 1
}
```

---

### Get Session
Get details of a specific session including messages.

**Endpoint:** `GET /sessions/{session_id}`

**Response:**
```json
{
  "id": "session_20260203_054213",
  "created_at": "2026-02-03T05:42:13-05:00",
  "updated_at": "2026-02-03T05:42:15-05:00",
  "message_count": 3,
  "user_messages": 1,
  "assistant_messages": 1,
  "tool_messages": 0,
  "working_dir": ".",
  "messages": [
    {"role": "system", "content": "You are a strategic AI assistant..."},
    {"role": "user", "content": "list files"},
    {"role": "assistant", "content": "Here are the files..."}
  ]
}
```

---

### Delete Session
Delete a specific session.

**Endpoint:** `DELETE /sessions/{session_id}`

**Response:**
```json
{
  "deleted": true,
  "id": "session_20260203_054213"
}
```

---

### Background Session
Move a session to background state.

**Endpoint:** `POST /sessions/{session_id}/background`

**Response:**
```json
{
  "id": "session_20260203_054213",
  "state": "background",
  "status": "ok"
}
```

---

### Activate Session
Activate a session for a client.

**Endpoint:** `POST /sessions/{session_id}/activate`

**Request Body:**
```json
{
  "client_id": "string (optional)"
}
```

**Response:**
```json
{
  "id": "session_20260203_054213",
  "state": "active",
  "client_id": "client-123",
  "status": "ok"
}
```

---

### Get Preferences
Get AI routing preferences.

**Endpoint:** `GET /preferences`

**Response:**
```json
{
  "fallback_order": ["deepseek", "kimi", "glm", "minimax", "qwen", "openai"],
  "consensus": {
    "workers": [...],
    "arbiter": [...]
  },
  "factory": {
    "specialists": {...},
    "guardrails": {...}
  },
  "default": {
    "provider": "deepseek",
    "model": "deepseek-chat"
  }
}
```

---

## Available AI Providers

### DeepSeek
- **Provider name:** `deepseek`
- **Default model:** `deepseek-chat`
- **Base URL:** `https://api.deepseek.com`
- **Models:** `deepseek-chat`, `deepseek-reasoner`

### Kimi (Moonshot)
- **Provider name:** `kimi`
- **Default model:** `kimi-k2-5`
- **Base URL:** `https://api.moonshot.cn/v1`
- **Context:** 256K tokens
- **Pricing:** $0.10/M input (with cache)
- **Models:** `kimi-k2-5`, `kimi-k2-5-long-context` (2M), `moonshot-v1-8k`, `moonshot-v1-32k`, `moonshot-v1-128k`

### Qwen (Alibaba Cloud)
- **Provider name:** `qwen`
- **Default model:** `qwen3-coder-30b-a3b-instruct`
- **Base URL:** `https://dashscope-us.aliyuncs.com/compatible-mode/v1`
- **Context:** 262K tokens
- **Models:** `qwen3-coder-30b-a3b-instruct`, `qwen-plus`, `qwen-max`, `qwen3-coder-480b-a35b-instruct`

### GLM (Zhipu AI)
- **Provider name:** `glm`
- **Default model:** `glm-4.7`
- **Base URL:** `https://open.bigmodel.cn/api/paas/v4`
- **Models:** `glm-4.7`, `glm-4`, `glm-3-turbo`

### Minimax
- **Provider name:** `minimax`
- **Default model:** `minimax-M2.1`
- **Base URL:** `https://api.minimax.chat/v1`
- **Models:** `minimax-M2.1`, `abab6.5s`, `abab6.5`

### OpenAI
- **Provider name:** `openai`
- **Default model:** `gpt-4o-mini`
- **Base URL:** `https://api.openai.com/v1`
- **Models:** `gpt-4o-mini`, `gpt-4o`, `gpt-4-turbo`, `gpt-3.5-turbo`

---

## Available Tools

### exec
Execute shell commands.
```json
{"command": "ls -la"}
```

### read_file
Read file contents.
```json
{"path": "README.md"}
```

### write_file
Create or overwrite files.
```json
{"path": "file.txt", "content": "...", "create_dirs": true}
```

### edit_file
String replacement in files.
```json
{"path": "file.go", "old_string": "...", "new_string": "...", "replace_all": false}
```

### append_file
Append content to file.
```json
{"path": "log.txt", "content": "new line\n"}
```

### list_dir
List directory contents.
```json
{"path": "."}
```

### search_files
Regex search in files.
```json
{"pattern": "func.*Error", "path": ".", "file_pattern": "*.go", "max_results": 50}
```

### system_info
Get system information.
```json
{}
```

---

## Error Responses

### HTTP Status Codes
- `200 OK`: Success
- `400 Bad Request`: Invalid parameters
- `404 Not Found`: Session not found
- `405 Method Not Allowed`: Wrong HTTP method
- `500 Internal Server Error`: Server error

### Error Format
```json
{
  "error": "Error message"
}
```

---

## Client Examples

### Go
```go
package main

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
)

func main() {
    req := map[string]interface{}{
        "user_input":  "list files",
        "working_dir": ".",
        "provider":    "deepseek",
    }
    
    jsonData, _ := json.Marshal(req)
    httpReq, _ := http.NewRequest("POST", "http://localhost:8080/chat/stream", bytes.NewBuffer(jsonData))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Accept", "text/event-stream")
    
    client := &http.Client{}
    resp, _ := client.Do(httpReq)
    defer resp.Body.Close()
    
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: ") {
            fmt.Println(line[6:])
        }
    }
}
```

### Python
```python
import requests
import json

url = "http://localhost:8080/chat/stream"
payload = {
    "user_input": "list files",
    "working_dir": ".",
    "provider": "deepseek"
}

response = requests.post(url, json=payload, stream=True, 
    headers={"Accept": "text/event-stream"})

for line in response.iter_lines():
    if line:
        line = line.decode('utf-8')
        if line.startswith('data: '):
            event = json.loads(line[6:])
            print(f"[{event.get('type')}] {event.get('message', '')}")
```

### curl
```bash
# Streaming
curl -N -X POST http://localhost:8080/chat/stream \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"user_input": "what is 2+2?", "working_dir": "/tmp"}'

# Blocking
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"user_input": "what is 2+2?"}'
```

---

## Configuration

### Environment Variables
```bash
export DEEPSEEK_API_KEY="sk-..."
export KIMI_API_KEY="sk-..."
export QWEN_API_KEY="sk-..."
export GLM_API_KEY="sk-..."
export MINIMAX_API_KEY="sk-..."
export OPENAI_API_KEY="sk-..."
```

### Config File
`~/.zen/zen-claw/config.yaml`

---

## Monitoring

```bash
# Health check
curl http://localhost:8080/health

# Active sessions
curl http://localhost:8080/sessions

# Gateway info
curl http://localhost:8080/
```
