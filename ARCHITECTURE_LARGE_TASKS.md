# Architecture for Large Tasks & Multi-AI Coordination

## Current Limitations

### 1. **Synchronous Request-Response Model**
- HTTP client timeout: 180s (hard limit)
- Gateway uses request context (cancels when client disconnects)
- No background task execution
- No task persistence/resumption

### 2. **Single Agent Execution**
- One agent, one task at a time
- Sequential tool execution
- No parallelization
- No sub-task delegation

### 3. **No Multi-AI Coordination**
- Single AI provider per request
- No AI-to-AI communication
- No task splitting across AIs
- No specialized AI roles (planner, executor, reviewer)

### 4. **Limited Context Management**
- Aggressive truncation (10-50 messages)
- No context sharing between AIs
- No context checkpointing
- No context summarization for long tasks

### 5. **No Task State Management**
- Tasks can't be paused/resumed
- No checkpointing
- No progress tracking
- No task queuing

## Required Macro Changes

### 1. **Async Task Execution System**

#### 1.1 Background Job Queue
```go
// New: internal/task/queue.go
type TaskQueue interface {
    Enqueue(task *Task) (string, error)  // Returns task ID
    Dequeue() (*Task, error)
    GetStatus(taskID string) (*TaskStatus, error)
    Cancel(taskID string) error
}

type Task struct {
    ID          string
    SessionID   string
    UserInput   string
    Provider    string
    Model       string
    Status      TaskStatus
    CreatedAt   time.Time
    StartedAt   *time.Time
    CompletedAt *time.Time
    Progress    TaskProgress
    Result      *TaskResult
    Error       string
}

type TaskStatus string
const (
    TaskStatusPending   TaskStatus = "pending"
    TaskStatusRunning   TaskStatus = "running"
    TaskStatusPaused    TaskStatus = "paused"
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusCanceled  TaskStatus = "canceled"
)
```

#### 1.2 Task Worker Pool
```go
// New: internal/task/worker.go
type WorkerPool struct {
    queue    TaskQueue
    workers  int
    executor TaskExecutor
}

func (wp *WorkerPool) Start() {
    for i := 0; i < wp.workers; i++ {
        go wp.workerLoop()
    }
}

func (wp *WorkerPool) workerLoop() {
    for {
        task := wp.queue.Dequeue()
        wp.executor.Execute(task)
    }
}
```

#### 1.3 Gateway API Changes
```go
// POST /tasks - Create async task (returns immediately)
// GET /tasks/{id} - Get task status
// GET /tasks/{id}/stream - Stream progress (SSE/WebSocket)
// POST /tasks/{id}/cancel - Cancel task
// POST /tasks/{id}/pause - Pause task
// POST /tasks/{id}/resume - Resume task
```

### 2. **Multi-AI Coordination System**

#### 2.1 AI Orchestrator
```go
// New: internal/orchestrator/orchestrator.go
type Orchestrator struct {
    providers map[string]ai.Provider
    router    *AIRouter
}

type CoordinationStrategy string
const (
    StrategySequential CoordinationStrategy = "sequential"  // One AI after another
    StrategyParallel   CoordinationStrategy = "parallel"    // Multiple AIs simultaneously
    StrategyHierarchical CoordinationStrategy = "hierarchical" // Planner -> Executors -> Reviewer
)

type TaskPlan struct {
    Steps []TaskStep
    Coordination CoordinationStrategy
    AIs []AIAssignment
}

type TaskStep struct {
    ID          string
    Description string
    AssignedAI  string  // Provider name
    Model       string
    Dependencies []string  // Step IDs that must complete first
    Context     map[string]interface{}
}

type AIAssignment struct {
    Provider    string
    Model       string
    Role        AIRole  // planner, executor, reviewer, specialist
    Capabilities []string
}

type AIRole string
const (
    RolePlanner    AIRole = "planner"     // Breaks down task
    RoleExecutor   AIRole = "executor"    // Executes subtasks
    RoleReviewer   AIRole = "reviewer"     // Reviews results
    RoleSpecialist AIRole = "specialist"  // Domain expert
)
```

#### 2.2 Task Decomposition
```go
// New: internal/orchestrator/decomposer.go
type TaskDecomposer interface {
    Decompose(task string, availableAIs []AIAssignment) (*TaskPlan, error)
}

// Example: Use Qwen for planning (large context), DeepSeek for execution (fast)
func (o *Orchestrator) PlanTask(task string) (*TaskPlan, error) {
    // 1. Use Qwen (planner) to break down task
    planReq := ai.ChatRequest{
        Model: "qwen3-coder-30b",
        Messages: []ai.Message{
            {Role: "system", Content: "You are a task planner. Break down complex tasks into steps."},
            {Role: "user", Content: task},
        },
    }
    
    planResp, err := o.providers["qwen"].Chat(ctx, planReq)
    // Parse planResp into TaskPlan
    
    // 2. Assign steps to appropriate AIs
    for _, step := range plan.Steps {
        step.AssignedAI = o.selectAI(step)
    }
    
    return plan, nil
}
```

#### 2.3 Context Sharing Between AIs
```go
// New: internal/orchestrator/context_manager.go
type ContextManager struct {
    store map[string]*SharedContext
}

type SharedContext struct {
    TaskID      string
    Summary     string  // High-level summary
    KeyFacts    []string
    Decisions   []Decision
    Artifacts   map[string]interface{}  // Files, results, etc.
    Messages    []ai.Message  // Condensed conversation
}

// Each AI can read/write to shared context
func (cm *ContextManager) GetContext(taskID string) *SharedContext
func (cm *ContextManager) UpdateContext(taskID string, updates *ContextUpdate)
func (cm *ContextManager) SummarizeContext(taskID string) string  // For new AIs joining
```

### 3. **Long-Running Task Support**

#### 3.1 Task Checkpointing
```go
// New: internal/task/checkpoint.go
type Checkpoint struct {
    TaskID      string
    Step        int
    State       map[string]interface{}
    Session     *agent.Session
    Timestamp   time.Time
}

type CheckpointManager interface {
    SaveCheckpoint(taskID string, checkpoint *Checkpoint) error
    LoadCheckpoint(taskID string) (*Checkpoint, error)
    ListCheckpoints(taskID string) ([]*Checkpoint, error)
}

// Agent can save state periodically
func (a *Agent) RunWithCheckpointing(ctx context.Context, task *Task) {
    for step := 0; step < a.maxSteps; step++ {
        // ... execute step ...
        
        // Save checkpoint every N steps
        if step % 10 == 0 {
            checkpoint := &Checkpoint{
                TaskID: task.ID,
                Step: step,
                State: a.getState(),
                Session: session,
            }
            checkpointManager.SaveCheckpoint(task.ID, checkpoint)
        }
    }
}
```

#### 3.2 Task Resumption
```go
// Resume from checkpoint
func (a *Agent) ResumeFromCheckpoint(taskID string) (*Session, error) {
    checkpoint, err := checkpointManager.LoadCheckpoint(taskID)
    if err != nil {
        return nil, err
    }
    
    // Restore session state
    session := checkpoint.Session
    
    // Continue from checkpoint step
    return a.Run(ctx, session, "Continue from checkpoint")
}
```

#### 3.3 Progress Streaming
```go
// New: internal/task/progress.go
type ProgressStream struct {
    taskID  string
    events  chan ProgressEvent
}

type ProgressEvent struct {
    TaskID    string
    Step      int
    Status    string
    Message   string
    Progress  float64  // 0.0 - 1.0
    Data      interface{}
}

// Gateway endpoint: GET /tasks/{id}/stream (SSE)
func (s *Server) streamTaskProgress(w http.ResponseWriter, r *http.Request) {
    taskID := extractTaskID(r)
    stream := progressManager.GetStream(taskID)
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    for event := range stream.Events() {
        fmt.Fprintf(w, "data: %s\n\n", json.Marshal(event))
        w.(http.Flusher).Flush()
    }
}
```

### 4. **Enhanced Context Management**

#### 4.1 Context Summarization
```go
// New: internal/context/summarizer.go
type ContextSummarizer interface {
    Summarize(messages []ai.Message, maxTokens int) (string, error)
    ExtractKeyFacts(messages []ai.Message) ([]string, error)
    ExtractDecisions(messages []ai.Message) ([]Decision, error)
}

// For long conversations, periodically summarize
func (cm *ContextManager) SummarizeLongContext(taskID string) {
    messages := cm.GetMessages(taskID)
    if len(messages) > 100 {
        summary := summarizer.Summarize(messages, 500)
        keyFacts := summarizer.ExtractKeyFacts(messages)
        
        cm.UpdateContext(taskID, &ContextUpdate{
            Summary: summary,
            KeyFacts: keyFacts,
        })
    }
}
```

#### 4.2 Smart Context Window Management
```go
// Instead of hard truncation, use intelligent context selection
type ContextSelector interface {
    SelectRelevant(messages []ai.Message, currentTask string, maxTokens int) []ai.Message
}

// Keep:
// - System message
// - Recent messages (last N)
// - Messages with tool calls (important state)
// - Messages referenced by current task
// - Summary of older messages
```

### 5. **Timeout & Resource Management**

#### 5.1 No Hard Timeouts for Background Tasks
```go
// Background tasks don't have HTTP client timeouts
// They run until completion or explicit cancellation

type TaskExecutor struct {
    timeout time.Duration  // Optional soft timeout (warns, doesn't kill)
    maxDuration time.Duration  // Hard limit (hours/days)
}

func (te *TaskExecutor) Execute(task *Task) {
    ctx := context.Background()  // No timeout!
    
    // Optional: Soft timeout warning
    go func() {
        time.Sleep(te.timeout)
        progressManager.Emit(task.ID, ProgressEvent{
            Message: "Task running longer than expected",
            Status: "warning",
        })
    }()
    
    // Hard limit check
    ctx, cancel := context.WithTimeout(ctx, te.maxDuration)
    defer cancel()
    
    agent.Run(ctx, session, task.UserInput)
}
```

#### 5.2 Resource Limits
```go
type ResourceLimits struct {
    MaxConcurrentTasks int
    MaxMemoryPerTask   int64
    MaxStepsPerTask    int
    MaxDurationPerTask time.Duration
}
```

### 6. **File Structure Changes**

```
zen-claw/
├── internal/
│   ├── task/              # NEW: Task management
│   │   ├── queue.go      # Task queue
│   │   ├── worker.go     # Worker pool
│   │   ├── executor.go   # Task executor
│   │   ├── checkpoint.go # Checkpointing
│   │   └── progress.go   # Progress tracking
│   ├── orchestrator/     # NEW: Multi-AI coordination
│   │   ├── orchestrator.go
│   │   ├── decomposer.go
│   │   ├── planner.go
│   │   └── context_manager.go
│   ├── context/           # NEW: Enhanced context
│   │   ├── summarizer.go
│   │   ├── selector.go
│   │   └── manager.go
│   └── ... (existing)
```

## Implementation Phases

### Phase 1: Async Task Execution (Foundation)
1. Implement TaskQueue (in-memory, then persistent)
2. Add background worker pool
3. Create `/tasks` API endpoints
4. Update gateway to support async execution
5. Add progress streaming (SSE)

### Phase 2: Task Persistence & Resumption
1. Implement checkpointing
2. Add task state persistence
3. Implement resume functionality
4. Add task history/audit log

### Phase 3: Multi-AI Coordination
1. Implement Orchestrator
2. Add task decomposition
3. Implement context sharing
4. Add AI role assignment
5. Test with 2-3 AIs coordinating

### Phase 4: Enhanced Context Management
1. Implement context summarization
2. Add smart context selection
3. Implement context checkpointing
4. Test with very long tasks (1000+ steps)

### Phase 5: Advanced Features
1. Task dependencies
2. Parallel execution
3. AI specialization
4. Cost optimization across AIs

## Migration Path

### Backward Compatibility
- Keep synchronous `/chat` endpoint
- Add new `/tasks` endpoint for async
- Clients can choose sync vs async
- Gradually migrate to async

### Configuration
```yaml
task:
  async_enabled: true
  max_concurrent: 10
  checkpoint_interval: 10  # steps
  max_duration: 24h
  
orchestration:
  enabled: true
  strategy: "hierarchical"  # sequential, parallel, hierarchical
  default_planner: "qwen"
  default_executor: "deepseek"
  default_reviewer: "qwen"
```

## Example: Large Task Flow

```
User: "Refactor entire codebase, add tests, update docs"

1. Task created → /tasks (async)
2. Returns task ID immediately
3. Background: Qwen (planner) decomposes:
   - Step 1: Analyze codebase structure (Qwen - large context)
   - Step 2: Refactor module A (DeepSeek - fast)
   - Step 3: Refactor module B (DeepSeek - fast)
   - Step 4: Add tests (Qwen - understands code)
   - Step 5: Update docs (DeepSeek - fast)
   - Step 6: Review changes (Qwen - large context)

4. Steps execute in parallel where possible
5. Context shared between AIs via ContextManager
6. Progress streamed to client
7. Checkpoints saved every 10 steps
8. If failure, resume from last checkpoint
9. Final result aggregated and returned
```

## Key Design Decisions

1. **No HTTP timeouts for background tasks** - They run until done
2. **Checkpointing is essential** - Long tasks need resumability
3. **Context summarization** - Enables very long conversations
4. **Multi-AI by design** - Not an afterthought
5. **Progress streaming** - Users need feedback on long tasks
6. **Backward compatible** - Don't break existing sync API
