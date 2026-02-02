# Session Attachment & Multi-Client Architecture

## Current State Analysis

### ✅ What Works
1. **Session Persistence**: Sessions are saved to disk (`/tmp/zen-claw-sessions/`)
2. **Session Retrieval**: Multiple clients can GET `/sessions/{id}` to read session state
3. **Session Listing**: Clients can list all sessions via `/sessions`

### ❌ What's Missing
1. **Background Task Execution**: Tasks stop when client disconnects
   - Current: `ctx := r.Context()` in `chatHandler` - cancels when HTTP client disconnects
   - Current: `agentInstance.Run(ctx, ...)` uses request context - stops on disconnect
   
2. **Client Connection Tracking**: No tracking of which clients are attached to which sessions
   - Can't notify multiple clients of progress
   - Can't detect when clients disconnect
   
3. **Task State Management**: No distinction between "session" and "active task"
   - Session exists, but no concept of "task running in background"
   - Can't query "what's currently running?"

4. **Progress Broadcasting**: No way to stream progress to multiple clients
   - Each client makes separate request
   - No real-time updates

## Required Architecture Changes

### 1. **Session vs Task Separation**

**Current Model:**
```
Session = Conversation history
Task = Synchronous execution tied to HTTP request
```

**Required Model:**
```
Session = Persistent conversation state (already exists ✅)
Task = Independent background execution (NEW)
  - Task has SessionID (belongs to session)
  - Task runs independently of client connections
  - Task persists state (checkpoints)
  - Multiple clients can attach to same task
```

### 2. **Background Task Execution**

#### 2.1 Task Lifecycle
```go
// Task states
type TaskStatus string
const (
    TaskStatusPending   TaskStatus = "pending"    // Queued, not started
    TaskStatusRunning   TaskStatus = "running"    // Currently executing
    TaskStatusPaused    TaskStatus = "paused"     // Paused by user
    TaskStatusCompleted TaskStatus = "completed"   // Finished successfully
    TaskStatusFailed    TaskStatus = "failed"      // Failed with error
    TaskStatusCanceled  TaskStatus = "canceled"    // Canceled by user
)

// Task continues even if all clients disconnect
// Gateway maintains task state independently
```

#### 2.2 Background Execution
```go
// internal/gateway/agent_service.go

// Chat - NEW: Creates async task, returns immediately
func (s *AgentService) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Create task (not execute immediately)
    task := &Task{
        ID:        generateTaskID(),
        SessionID: req.SessionID,
        UserInput: req.UserInput,
        Provider:  req.Provider,
        Model:     req.Model,
        Status:    TaskStatusPending,
        CreatedAt: time.Now(),
    }
    
    // Save task
    s.taskStore.SaveTask(task)
    
    // Enqueue for background execution
    s.taskQueue.Enqueue(task)
    
    // Return immediately (don't wait for execution)
    return &ChatResponse{
        SessionID: req.SessionID,
        TaskID:    task.ID,  // NEW: Return task ID
        Status:    "queued",
    }, nil
}

// ExecuteTask - Background worker executes this
func (s *AgentService) ExecuteTask(task *Task) {
    // Update status
    task.Status = TaskStatusRunning
    task.StartedAt = time.Now()
    s.taskStore.SaveTask(task)
    
    // Get session
    session := s.getOrCreateSession(task.SessionID)
    
    // Create agent
    aiCaller := &GatewayAICaller{
        aiRouter: s.aiRouter,
        provider: task.Provider,
        model:    task.Model,
    }
    agentInstance := agent.NewAgent(aiCaller, s.tools, 1000) // High max steps
    
    // Execute with background context (NOT request context!)
    bgCtx := context.Background()  // No timeout, no cancellation from client
    
    // Run agent
    updatedSession, result, err := agentInstance.Run(bgCtx, session, task.UserInput)
    
    // Update task status
    if err != nil {
        task.Status = TaskStatusFailed
        task.Error = err.Error()
    } else {
        task.Status = TaskStatusCompleted
        task.Result = result
    }
    task.CompletedAt = time.Now()
    s.taskStore.SaveTask(task)
    
    // Save session
    s.sessionStore.SaveSession(updatedSession)
    
    // Broadcast completion to all attached clients
    s.broadcastTaskUpdate(task)
}
```

### 3. **Client Attachment Tracking**

#### 3.1 Client Registry
```go
// internal/gateway/client_registry.go
type ClientRegistry struct {
    // Map: sessionID -> []ClientConnection
    attachments map[string][]*ClientConnection
    mu          sync.RWMutex
}

type ClientConnection struct {
    ID          string
    Type        ClientType  // "cli", "web", "slack", "telegram"
    SessionID   string
    TaskID      string     // Optional: attached to specific task
    ConnectedAt time.Time
    LastSeen    time.Time
    SendChan    chan TaskUpdate  // Channel to send updates
}

type ClientType string
const (
    ClientTypeCLI      ClientType = "cli"
    ClientTypeWeb      ClientType = "web"
    ClientTypeSlack    ClientType = "slack"
    ClientTypeTelegram ClientType = "telegram"
)

// Attach client to session
func (cr *ClientRegistry) Attach(clientID string, clientType ClientType, sessionID string) *ClientConnection {
    conn := &ClientConnection{
        ID:          clientID,
        Type:        clientType,
        SessionID:   sessionID,
        ConnectedAt: time.Now(),
        LastSeen:    time.Now(),
        SendChan:    make(chan TaskUpdate, 100),
    }
    
    cr.mu.Lock()
    cr.attachments[sessionID] = append(cr.attachments[sessionID], conn)
    cr.mu.Unlock()
    
    return conn
}

// Detach client (on disconnect)
func (cr *ClientRegistry) Detach(clientID string) {
    cr.mu.Lock()
    defer cr.mu.Unlock()
    
    for sessionID, clients := range cr.attachments {
        for i, client := range clients {
            if client.ID == clientID {
                // Remove client
                cr.attachments[sessionID] = append(clients[:i], clients[i+1:]...)
                close(client.SendChan)
                return
            }
        }
    }
}

// Broadcast to all clients attached to session
func (cr *ClientRegistry) Broadcast(sessionID string, update TaskUpdate) {
    cr.mu.RLock()
    clients := cr.attachments[sessionID]
    cr.mu.RUnlock()
    
    for _, client := range clients {
        select {
        case client.SendChan <- update:
        default:
            // Channel full, skip (client might be slow/disconnected)
        }
    }
}
```

#### 3.2 Gateway API Updates
```go
// POST /sessions/{id}/attach - Attach to session (returns connection ID)
// GET /sessions/{id}/stream - Stream updates (SSE/WebSocket)
// POST /sessions/{id}/detach - Detach from session
// GET /tasks/{id} - Get task status
// GET /tasks/{id}/stream - Stream task progress
```

### 4. **Multi-Client Progress Streaming**

#### 4.1 Progress Events
```go
type TaskUpdate struct {
    TaskID    string      `json:"task_id"`
    SessionID string      `json:"session_id"`
    Status    TaskStatus  `json:"status"`
    Step      int         `json:"step,omitempty"`
    Message   string      `json:"message,omitempty"`
    Progress  float64     `json:"progress,omitempty"`  // 0.0 - 1.0
    Result    string      `json:"result,omitempty"`
    Error     string      `json:"error,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
}

// Agent emits progress events
func (a *Agent) Run(ctx context.Context, session *Session, userInput string) {
    for step := 0; step < a.maxSteps; step++ {
        // ... execute step ...
        
        // Emit progress event
        progress := TaskUpdate{
            TaskID:    taskID,
            SessionID: session.ID,
            Status:    TaskStatusRunning,
            Step:      step,
            Message:   fmt.Sprintf("Step %d: %s", step, currentAction),
            Progress:  float64(step) / float64(a.maxSteps),
        }
        
        // Send to progress broadcaster
        progressBroadcaster.Broadcast(session.ID, progress)
    }
}
```

#### 4.2 SSE Endpoint
```go
// GET /sessions/{sessionID}/stream
func (s *Server) streamSessionUpdates(w http.ResponseWriter, r *http.Request) {
    sessionID := extractSessionID(r)
    clientID := generateClientID()
    clientType := extractClientType(r) // From header or query param
    
    // Attach client
    conn := s.clientRegistry.Attach(clientID, clientType, sessionID)
    defer s.clientRegistry.Detach(clientID)
    
    // Set up SSE
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
    
    // Send initial connection confirmation
    fmt.Fprintf(w, "data: %s\n\n", json.Marshal(TaskUpdate{
        Message: "Connected to session",
        Status:  "connected",
    }))
    w.(http.Flusher).Flush()
    
    // Stream updates
    ticker := time.NewTicker(30 * time.Second) // Keep-alive
    defer ticker.Stop()
    
    for {
        select {
        case update := <-conn.SendChan:
            fmt.Fprintf(w, "data: %s\n\n", json.Marshal(update))
            w.(http.Flusher).Flush()
            
        case <-ticker.C:
            // Keep-alive ping
            fmt.Fprintf(w, ": ping\n\n")
            w.(http.Flusher).Flush()
            
        case <-r.Context().Done():
            // Client disconnected
            return
        }
    }
}
```

### 5. **Task Continuation on Disconnect**

#### 5.1 Key Principle
```
✅ Gateway owns task execution
✅ Tasks run in background goroutines
✅ Tasks use context.Background() (no client cancellation)
✅ Client disconnection = detach from updates, NOT cancel task
✅ Tasks continue until completion, failure, or explicit cancel
```

#### 5.2 Implementation
```go
// Task worker pool (runs in background, independent of HTTP requests)
type TaskWorkerPool struct {
    queue    TaskQueue
    executor *TaskExecutor
    workers  int
}

func (twp *TaskWorkerPool) Start() {
    for i := 0; i < twp.workers; i++ {
        go twp.workerLoop()
    }
}

func (twp *TaskWorkerPool) workerLoop() {
    for {
        task := twp.queue.Dequeue()
        
        // Execute in background (no HTTP context!)
        go func(t *Task) {
            twp.executor.Execute(t)
        }(task)
    }
}

// TaskExecutor uses background context
func (te *TaskExecutor) Execute(task *Task) {
    // Background context - NOT tied to any HTTP request
    ctx := context.Background()
    
    // Optional: Add soft timeout (warns but doesn't cancel)
    ctx, cancel := context.WithTimeout(ctx, 24*time.Hour) // Very long limit
    defer cancel()
    
    // Execute task
    agent.Run(ctx, session, task.UserInput)
    
    // Task completes regardless of client connections
}
```

### 6. **Client Reconnection**

#### 6.1 Reconnect Flow
```
1. Client disconnects (network issue, app closed, etc.)
2. Gateway detaches client from session
3. Task continues running in background
4. Client reconnects: POST /sessions/{id}/attach
5. Gateway sends current task status
6. Client receives missed updates + future updates
```

#### 6.2 Catch-Up on Reconnect
```go
// When client attaches, send current state
func (s *Server) attachToSession(w http.ResponseWriter, r *http.Request) {
    sessionID := extractSessionID(r)
    
    // Get current task for session
    task := s.taskStore.GetActiveTask(sessionID)
    
    // Get session state
    session, _ := s.sessionStore.GetSession(sessionID)
    
    // Send current state
    response := AttachResponse{
        SessionID: sessionID,
        TaskID:    task.ID,
        Status:    task.Status,
        Progress:  task.Progress,
        Messages:  session.GetMessages(), // Full conversation history
    }
    
    json.NewEncoder(w).Encode(response)
    
    // Client can then connect to stream for future updates
}
```

## API Design

### New Endpoints

```
# Task Management
POST   /tasks                    # Create async task (returns immediately)
GET    /tasks/{id}               # Get task status
GET    /tasks/{id}/stream        # Stream task progress (SSE)
POST   /tasks/{id}/cancel        # Cancel task
POST   /tasks/{id}/pause         # Pause task
POST   /tasks/{id}/resume        # Resume task

# Session Attachment
POST   /sessions/{id}/attach     # Attach client to session
GET    /sessions/{id}/stream     # Stream session updates (SSE)
POST   /sessions/{id}/detach     # Detach client from session
GET    /sessions/{id}/clients    # List clients attached to session

# Backward Compatible
POST   /chat                     # Sync execution (for simple tasks)
                                  # OR: Creates async task if async=true param
```

### Request/Response Examples

#### Create Async Task
```bash
POST /tasks
{
  "session_id": "session_123",
  "user_input": "Refactor entire codebase",
  "provider": "qwen",
  "model": "qwen3-coder-30b"
}

# Response (immediate)
{
  "task_id": "task_456",
  "session_id": "session_123",
  "status": "pending",
  "created_at": "2026-02-02T15:00:00Z"
}
```

#### Attach to Session
```bash
POST /sessions/session_123/attach
Headers: X-Client-Type: cli, X-Client-ID: cli_789

# Response
{
  "connection_id": "conn_abc",
  "session_id": "session_123",
  "active_task": {
    "task_id": "task_456",
    "status": "running",
    "step": 42,
    "progress": 0.42
  },
  "messages": [...]  # Full conversation
}
```

#### Stream Updates
```bash
GET /sessions/session_123/stream
Headers: X-Client-ID: cli_789

# SSE Stream
data: {"task_id":"task_456","status":"running","step":43,"progress":0.43,"message":"Step 43: Refactoring module X"}

data: {"task_id":"task_456","status":"running","step":44,"progress":0.44,"message":"Step 44: Writing tests"}

data: {"task_id":"task_456","status":"completed","progress":1.0,"result":"Refactoring complete"}
```

## Implementation Checklist

### Phase 1: Background Task Execution
- [ ] Create TaskQueue interface and implementation
- [ ] Create TaskStore for persistence
- [ ] Modify Chat() to create async tasks
- [ ] Create background worker pool
- [ ] Use context.Background() for task execution
- [ ] Test: Task continues after client disconnect

### Phase 2: Client Attachment
- [ ] Create ClientRegistry
- [ ] Implement attach/detach logic
- [ ] Add client tracking to sessions
- [ ] Test: Multiple clients attach to same session

### Phase 3: Progress Streaming
- [ ] Implement TaskUpdate events
- [ ] Add progress emission in Agent.Run()
- [ ] Create SSE endpoint for streaming
- [ ] Test: Clients receive real-time updates

### Phase 4: Reconnection
- [ ] Implement catch-up on reconnect
- [ ] Send current state to reconnecting clients
- [ ] Handle missed updates gracefully
- [ ] Test: Client reconnects and sees progress

## Alignment Verification

✅ **Sessions persist independently** - Already implemented (SessionStore)
✅ **Multiple clients can attach** - Architecture supports (ClientRegistry)
✅ **Gateway continues on disconnect** - Background tasks (TaskWorkerPool)
✅ **AIs continue working** - Background context (context.Background())
✅ **Clients can reconnect** - Reconnection flow with catch-up
✅ **Multi-platform support** - ClientType abstraction (CLI, Web, Slack, Telegram)

## Key Design Decisions

1. **Tasks are independent of clients** - Gateway owns execution
2. **Sessions are shared state** - Multiple clients can read/write
3. **Progress is broadcast** - All attached clients get updates
4. **Disconnect = detach, not cancel** - Tasks continue running
5. **Reconnect = catch-up** - Clients get current state + future updates
6. **Backward compatible** - Keep `/chat` endpoint for sync execution
