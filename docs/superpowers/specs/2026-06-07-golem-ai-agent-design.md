# Golem AI Agent Design Specification

**Date**: 2026-06-07
**Status**: Draft
**Author**: Claude Code (Brainstorming)

## 1. Overview

### 1.1 Project Goal

Build a local AI agent framework in Go that:
- Supports multi-turn conversations with streaming output
- Integrates with Feishu/Lark bot for messaging
- Provides plugin architecture for extensibility
- Supports multiple LLM providers (OpenAI, Claude, Gemini)
- Enables future expansion to other IM channels (WeChat, Telegram, etc.)

### 1.2 Key Design Principles

1. **Plugin-First Architecture**: All channels and LLM providers are plugins
2. **Single Binary Deployment**: Compile to a single Go binary
3. **Interface-Driven Design**: Go interfaces for all extensible components
4. **Event-Driven Communication**: Components communicate via event bus
5. **Configuration-Driven**: YAML-based configuration with sensible defaults

### 1.3 Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.21+ |
| Feishu SDK | oapi-sdk-go | v3 |
| HTTP Client | net/http (stdlib) | - |
| Configuration | gopkg.in/yaml.v3 | v3 |
| Logging | log/slog (stdlib) | Go 1.21+ |
| Testing | testing + testify | - |
| Streaming | SSE (Server-Sent Events) | - |

## 2. Architecture

### 2.1 System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Golem AI Agent                             │
├─────────────────────────────────────────────────────────────────┤
│                        Core Layer                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │Engine        │  │SessionManager│  │EventBus              │  │
│  │              │  │              │  │                      │  │
│  │- Lifecycle   │  │- Create      │  │- Publish             │  │
│  │- Orchestrate │  │- Get         │  │- Subscribe           │  │
│  │- Shutdown    │  │- Delete      │  │- Async dispatch      │  │
│  │              │  │- History     │  │                      │  │
│  └──────────────┘  └──────────────┘  └──────────────────────┘  │
├─────────────────────────────────────────────────────────────────┤
│                      Plugin Layer                               │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                 PluginManager                            │  │
│  │  - Load/Unload plugins                                   │  │
│  │  - Health check                                          │  │
│  │  - Dependency resolution                                 │  │
│  └──────────────────────────────────────────────────────────┘  │
│       │                      │                      │          │
│       ▼                      ▼                      ▼          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────┐    │
│  │  Channel    │    │  Provider   │    │  Tool           │    │
│  │  Plugins    │    │  Plugins    │    │  Plugins        │    │
│  ├─────────────┤    ├─────────────┤    ├─────────────────┤    │
│  │ FeishuPlugin│    │OpenAIPlugin │    │ (Future)        │    │
│  │ WeChatPlugin│    │ClaudePlugin │    │ WebSearch       │    │
│  │ Telegram    │    │GeminiPlugin │    │ FileOps         │    │
│  └─────────────┘    └─────────────┘    └─────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 Component Interaction Flow

```
User Message (Feishu)
       │
       ▼
┌──────────────┐
│FeishuPlugin  │
│(Channel)     │
└──────┬───────┘
       │ MessageEvent
       ▼
┌──────────────┐
│  EventBus    │
└──────┬───────┘
       │
       ▼
┌──────────────┐
│Engine        │
│(Core)        │
└──────┬───────┘
       │
       ├──────────────────────┐
       ▼                      ▼
┌──────────────┐    ┌──────────────┐
│SessionManager│    │ProviderPlugin│
│              │    │(e.g. OpenAI) │
│- Get session │    │- Chat        │
│- Add message │    │- Stream      │
│- Get history │    └──────┬───────┘
└──────────────┘           │
       │                   │ StreamDelta events
       │                   ▼
       │           ┌──────────────┐
       │           │  EventBus    │
       │           └──────┬───────┘
       │                  │
       │                  ▼
       │           ┌──────────────┐
       └──────────►│FeishuPlugin  │
                   │(Reply)       │
                   └──────────────┘
```

## 3. Core Components

### 3.1 Engine

The Engine is the central orchestrator that manages the application lifecycle.

```go
// internal/core/engine.go

type Engine struct {
    config         *config.Config
    sessionMgr     *SessionManager
    eventBus       *EventBus
    pluginMgr      *plugin.Manager
    logger         *slog.Logger
    ctx            context.Context
    cancel         context.CancelFunc
}

type EngineInterface interface {
    Start() error
    Shutdown() error
    GetSessionManager() SessionManagerInterface
    GetEventBus() EventBusInterface
    GetPluginManager() plugin.ManagerInterface
}
```

**Responsibilities:**
- Initialize and wire all components
- Start/stop plugins
- Handle graceful shutdown
- Provide component access

### 3.2 SessionManager

Manages conversation sessions with history and context.

```go
// internal/core/session.go

type Session struct {
    ID        string
    UserID    string
    Channel   string
    Messages  []Message
    Metadata  map[string]interface{}
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Message struct {
    Role      string    // "user", "assistant", "system"
    Content   string
    Timestamp time.Time
    Metadata  map[string]interface{}
}

type SessionManager struct {
    sessions map[string]*Session
    mu       sync.RWMutex
    config   SessionConfig
    logger   *slog.Logger
}

type SessionManagerInterface interface {
    CreateSession(userID, channel string) (*Session, error)
    GetSession(sessionID string) (*Session, error)
    AddMessage(sessionID string, msg Message) error
    GetHistory(sessionID string, limit int) ([]Message, error)
    DeleteSession(sessionID string) error
    CleanupStale(maxAge time.Duration) int
}
```

**Session ID Format:**
- Direct message: `{channel}:{userID}`
- Group message: `{channel}:group:{chatID}:{userID}`

**Configuration:**
```yaml
session:
  max_history: 50        # Maximum messages per session
  trim_to: 20            # Trim to this many when max reached
  idle_timeout: 30m      # Session idle timeout
  cleanup_interval: 5m   # Background cleanup interval
```

### 3.3 EventBus

Pub/sub event system for decoupled component communication.

```go
// internal/core/event.go

type EventType string

const (
    EventMessageReceived  EventType = "message.received"
    EventMessageSent      EventType = "message.sent"
    EventStreamDelta      EventType = "stream.delta"
    EventStreamDone       EventType = "stream.done"
    EventStreamError      EventType = "stream.error"
    EventSessionCreated   EventType = "session.created"
    EventSessionDeleted   EventType = "session.deleted"
    EventPluginLoaded     EventType = "plugin.loaded"
    EventPluginUnloaded   EventType = "plugin.unloaded"
)

type Event struct {
    Type      EventType
    Source    string
    Data      interface{}
    Timestamp time.Time
}

type EventHandler func(event Event) error

type EventBus struct {
    handlers map[EventType][]EventHandler
    mu       sync.RWMutex
    logger   *slog.Logger
}

type EventBusInterface interface {
    Publish(event Event) error
    Subscribe(eventType EventType, handler EventHandler) error
    Unsubscribe(eventType EventType, handler EventHandler) error
}
```

**Event Flow Example (Streaming Response):**
1. `EventMessageReceived` → Engine processes message
2. Engine calls ProviderPlugin.ChatStream()
3. ProviderPlugin publishes `EventStreamDelta` for each chunk
4. ChannelPlugin subscribes to `EventStreamDelta` and sends to user
5. ProviderPlugin publishes `EventStreamDone` when complete

### 3.4 ConfigManager

YAML-based configuration with environment variable support.

```go
// internal/config/config.go

type Config struct {
    Server   ServerConfig   `yaml:"server"`
    LLM      LLMConfig      `yaml:"llm"`
    Session  SessionConfig  `yaml:"session"`
    Plugins  PluginsConfig  `yaml:"plugins"`
    Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
    Host string `yaml:"host"`
    Port int    `yaml:"port"`
}

type LLMConfig struct {
    DefaultProvider string                    `yaml:"default_provider"`
    Providers       map[string]ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
    APIKey      string  `yaml:"api_key"`
    BaseURL     string  `yaml:"base_url"`
    Model       string  `yaml:"model"`
    Temperature float64 `yaml:"temperature"`
    MaxTokens   int     `yaml:"max_tokens"`
}

type SessionConfig struct {
    MaxHistory       int           `yaml:"max_history"`
    TrimTo           int           `yaml:"trim_to"`
    IdleTimeout      time.Duration `yaml:"idle_timeout"`
    CleanupInterval  time.Duration `yaml:"cleanup_interval"`
}

type PluginsConfig struct {
    Channels map[string]map[string]interface{} `yaml:"channels"`
    Providers map[string]map[string]interface{} `yaml:"providers"`
}

type LoggingConfig struct {
    Level  string `yaml:"level"`
    Format string `yaml:"format"`
}
```

**Example Configuration (golem.yaml):**
```yaml
server:
  host: "0.0.0.0"
  port: 9921

llm:
  default_provider: "openai"
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
      base_url: "https://api.openai.com/v1"
      model: "gpt-4o"
      temperature: 0.7
      max_tokens: 4096
    claude:
      api_key: "${ANTHROPIC_API_KEY}"
      base_url: "https://api.anthropic.com"
      model: "claude-3-opus-20240229"
      temperature: 0.7
      max_tokens: 4096

session:
  max_history: 50
  trim_to: 20
  idle_timeout: 30m
  cleanup_interval: 5m

plugins:
  channels:
    feishu:
      app_id: "${FEISHU_APP_ID}"
      app_secret: "${FEISHU_APP_SECRET}"
      verification_token: "${FEISHU_VERIFICATION_TOKEN}"

logging:
  level: "info"
  format: "json"
```

## 4. Plugin System

### 4.1 Plugin Interfaces

```go
// internal/plugin/interfaces.go

// Plugin is the base interface for all plugins
type Plugin interface {
    // Name returns the unique plugin name
    Name() string

    // Version returns the plugin version
    Version() string

    // Initialize initializes the plugin with configuration
    Initialize(config map[string]interface{}) error

    // Start starts the plugin
    Start() error

    // Stop stops the plugin gracefully
    Stop() error

    // HealthCheck returns the plugin health status
    HealthCheck() HealthStatus
}

type HealthStatus struct {
    Healthy bool
    Message string
}

// ChannelPlugin extends Plugin for messaging channels
type ChannelPlugin interface {
    Plugin

    // SendMessage sends a message through the channel
    SendMessage(sessionID string, content string) error

    // SendStreamingMessage sends a streaming message
    SendStreamingMessage(sessionID string, stream <-chan StreamChunk) error

    // GetChannelType returns the channel type identifier
    GetChannelType() string
}

// ProviderPlugin extends Plugin for LLM providers
type ProviderPlugin interface {
    Plugin

    // Chat sends a message and returns a response
    Chat(ctx context.Context, messages []Message, config ChatConfig) (*ChatResponse, error)

    // ChatStream sends a message and returns a streaming response
    ChatStream(ctx context.Context, messages []Message, config ChatConfig) (<-chan StreamChunk, error)

    // GetProviderType returns the provider type identifier
    GetProviderType() string

    // SupportsStreaming returns whether the provider supports streaming
    SupportsStreaming() bool
}

// ToolPlugin extends Plugin for tool capabilities
type ToolPlugin interface {
    Plugin

    // Execute executes the tool with given parameters
    Execute(ctx context.Context, params map[string]interface{}) (interface{}, error)

    // GetToolDefinition returns the tool definition for function calling
    GetToolDefinition() ToolDefinition
}

type StreamChunk struct {
    Content string
    Done    bool
    Error   error
}

type ChatConfig struct {
    Model       string
    Temperature float64
    MaxTokens   int
    Stream      bool
}

type ChatResponse struct {
    Content string
    Usage   Usage
}

type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}

type ToolDefinition struct {
    Name        string
    Description string
    Parameters  map[string]interface{}
}
```

### 4.2 PluginManager

```go
// internal/plugin/manager.go

type Manager struct {
    plugins    map[string]Plugin
    channels   map[string]ChannelPlugin
    providers  map[string]ProviderPlugin
    tools      map[string]ToolPlugin
    mu         sync.RWMutex
    logger     *slog.Logger
}

type ManagerInterface interface {
    // LoadPlugin loads a plugin by name
    LoadPlugin(name string, plugin Plugin) error

    // UnloadPlugin unloads a plugin by name
    UnloadPlugin(name string) error

    // GetPlugin returns a plugin by name
    GetPlugin(name string) (Plugin, bool)

    // GetChannel returns a channel plugin by type
    GetChannel(channelType string) (ChannelPlugin, bool)

    // GetProvider returns a provider plugin by type
    GetProvider(providerType string) (ProviderPlugin, bool)

    // GetTool returns a tool plugin by name
    GetTool(name string) (ToolPlugin, bool)

    // ListPlugins returns all loaded plugins
    ListPlugins() []PluginInfo

    // StartAll starts all loaded plugins
    StartAll() error

    // StopAll stops all loaded plugins
    StopAll() error

    // HealthCheckAll checks health of all plugins
    HealthCheckAll() map[string]HealthStatus
}

type PluginInfo struct {
    Name    string
    Version string
    Type    string // "channel", "provider", "tool"
    Status  string // "loaded", "started", "stopped", "error"
}
```

### 4.3 Plugin Registry

```go
// internal/plugin/registry.go

type Registry struct {
    factories map[string]PluginFactory
    mu        sync.RWMutex
}

type PluginFactory func() Plugin

// Register registers a plugin factory
func Register(name string, factory PluginFactory) {
    globalRegistry.Register(name, factory)
}

// Create creates a plugin instance by name
func Create(name string) (Plugin, error) {
    return globalRegistry.Create(name)
}
```

## 5. Channel Plugins

### 5.1 FeishuPlugin

```go
// plugins/channels/feishu/plugin.go

type FeishuPlugin struct {
    config      FeishuConfig
    client      *lark.Client
    wsClient    *larkws.Client
    eventBus    core.EventBusInterface
    logger      *slog.Logger
    dedup       *Deduplicator
    streaming   *StreamingManager
}

type FeishuConfig struct {
    AppID             string `yaml:"app_id"`
    AppSecret         string `yaml:"app_secret"`
    VerificationToken string `yaml:"verification_token"`
    EncryptKey        string `yaml:"encrypt_key"`
}

func (p *FeishuPlugin) Name() string { return "feishu" }
func (p *FeishuPlugin) Version() string { return "1.0.0" }

func (p *FeishuPlugin) Initialize(config map[string]interface{}) error {
    // Parse config
    // Create Lark client
    // Create WebSocket client
    // Initialize deduplicator
    // Initialize streaming manager
}

func (p *FeishuPlugin) Start() error {
    // Register event handlers
    // Start WebSocket connection
    // Subscribe to EventBus events
}

func (p *FeishuPlugin) Stop() error {
    // Close WebSocket connection
    // Unregister event handlers
}
```

### 5.2 Message Handling Flow

```go
// plugins/channels/feishu/handler.go

type MessageHandler struct {
    plugin   *FeishuPlugin
    engine   core.EngineInterface
}

func (h *MessageHandler) HandleMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
    // 1. Deduplication check
    if h.plugin.dedup.IsDuplicate(event) {
        return nil
    }

    // 2. Extract message content
    userID := event.Sender.SenderId.OpenId
    chatID := event.Message.ChatId
    content := extractTextContent(event)

    // 3. Generate session ID
    sessionID := generateSessionID(event)

    // 4. Get or create session
    session, err := h.engine.GetSessionManager().GetOrCreateSession(sessionID, userID, "feishu")
    if err != nil {
        return err
    }

    // 5. Add user message to session
    h.engine.GetSessionManager().AddMessage(sessionID, core.Message{
        Role:    "user",
        Content: content,
    })

    // 6. Get LLM provider
    provider, err := h.engine.GetPluginManager().GetProvider(h.engine.GetConfig().LLM.DefaultProvider)
    if err != nil {
        return err
    }

    // 7. Get session history
    history, _ := h.engine.GetSessionManager().GetHistory(sessionID, 50)

    // 8. Call provider with streaming
    stream, err := provider.ChatStream(ctx, history, core.ChatConfig{
        Stream: true,
    })
    if err != nil {
        return err
    }

    // 9. Handle streaming response
    return h.handleStreamingResponse(sessionID, stream)
}

func (h *MessageHandler) handleStreamingResponse(sessionID string, stream <-chan core.StreamChunk) error {
    // Create streaming message
    // Forward chunks to Feishu
    // Update session with assistant message
}

func generateSessionID(event *larkim.P2MessageReceiveV1) string {
    if event.Message.ChatType == "p2p" {
        return fmt.Sprintf("feishu:%s", event.Sender.SenderId.OpenId)
    }
    return fmt.Sprintf("feishu:group:%s:%s", event.Message.ChatId, event.Sender.SenderId.OpenId)
}
```

### 5.3 Streaming Output

```go
// plugins/channels/feishu/streaming.go

type StreamingManager struct {
    plugin       *FeishuPlugin
    throttle     *Throttle
    cardBuilder  *CardBuilder
}

type Throttle struct {
    minInterval  time.Duration
    minChars     int
    lastUpdate   time.Time
    buffer       string
}

func (sm *StreamingManager) CreateStreamingMessage(sessionID, content string) (string, error) {
    // Create initial Card Kit message
    // Return message ID for updates
}

func (sm *StreamingManager) UpdateStreamingMessage(messageID, content string) error {
    // Throttle updates (160ms min interval, 18 chars min change)
    // Update Card Kit message
}

func (sm *StreamingManager) FinalizeStreamingMessage(messageID, content string) error {
    // Final update with complete content
    // Mark as complete
}
```

## 6. Provider Plugins

### 6.1 OpenAIProvider

```go
// plugins/providers/openai/provider.go

type OpenAIProvider struct {
    config     ProviderConfig
    httpClient *http.Client
    logger     *slog.Logger
}

type ProviderConfig struct {
    APIKey      string
    BaseURL     string
    Model       string
    Temperature float64
    MaxTokens   int
}

func (p *OpenAIProvider) Name() string { return "openai" }
func (p *OpenAIProvider) Version() string { return "1.0.0" }

func (p *OpenAIProvider) Chat(ctx context.Context, messages []core.Message, config core.ChatConfig) (*core.ChatResponse, error) {
    // Build request
    // Call API
    // Parse response
    // Return ChatResponse
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error) {
    // Build request with stream=true
    // Call API
    // Parse SSE response
    // Return channel of StreamChunk
}
```

### 6.2 Streaming Response Parsing

```go
// plugins/providers/openai/streaming.go

type StreamParser struct {
    reader *bufio.Reader
}

func (p *StreamParser) Parse() <-chan core.StreamChunk {
    chunks := make(chan core.StreamChunk)

    go func() {
        defer close(chunks)

        for {
            line, err := p.reader.ReadString('\n')
            if err != nil {
                if err != io.EOF {
                    chunks <- core.StreamChunk{Error: err}
                }
                return
            }

            line = strings.TrimSpace(line)
            if !strings.HasPrefix(line, "data: ") {
                continue
            }

            data := strings.TrimPrefix(line, "data: ")
            if data == "[DONE]" {
                chunks <- core.StreamChunk{Done: true}
                return
            }

            var chunk ChatCompletionChunk
            if err := json.Unmarshal([]byte(data), &chunk); err != nil {
                continue
            }

            if len(chunk.Choices) > 0 {
                chunks <- core.StreamChunk{
                    Content: chunk.Choices[0].Delta.Content,
                }
            }
        }
    }()

    return chunks
}
```

### 6.3 ClaudeProvider

```go
// plugins/providers/claude/provider.go

type ClaudeProvider struct {
    config     ProviderConfig
    httpClient *http.Client
    logger     *slog.Logger
}

func (p *ClaudeProvider) Chat(ctx context.Context, messages []core.Message, config core.ChatConfig) (*core.ChatResponse, error) {
    // Convert messages to Claude format
    // Call Anthropic API
    // Parse response
}

func (p *ClaudeProvider) ChatStream(ctx context.Context, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error) {
    // Call Anthropic streaming API
    // Parse SSE response
    // Return channel
}
```

## 7. Data Flow

### 7.1 User Message Flow

```
1. User sends message via Feishu
          │
          ▼
2. FeishuPlugin receives WebSocket event
          │
          ▼
3. MessageHandler processes message
   - Deduplication check
   - Extract content
   - Generate session ID
          │
          ▼
4. SessionManager.GetOrCreateSession()
   - Get existing or create new session
          │
          ▼
5. SessionManager.AddMessage()
   - Add user message to history
   - Trim if necessary
          │
          ▼
6. ProviderPlugin.ChatStream()
   - Call LLM API with history
   - Returns <-chan StreamChunk
          │
          ▼
7. StreamingManager handles response
   - Create streaming message
   - Forward chunks to Feishu
   - Throttle updates
          │
          ▼
8. SessionManager.AddMessage()
   - Add assistant message to history
          │
          ▼
9. User receives streaming response
```

### 7.2 Event Flow

```
EventMessageReceived
    │
    ├─► Engine processes message
    │
    ├─► SessionManager updates session
    │
    └─► ProviderPlugin.ChatStream()
            │
            ├─► EventStreamDelta (multiple)
            │       │
            │       └─► ChannelPlugin.SendStreamingMessage()
            │
            └─► EventStreamDone
                    │
                    └─► SessionManager.AddMessage(assistant)
```

## 8. Error Handling

### 8.1 Error Types

```go
// internal/core/errors.go

type ErrorCode string

const (
    ErrCodeSessionNotFound  ErrorCode = "SESSION_NOT_FOUND"
    ErrCodeProviderFailed   ErrorCode = "PROVIDER_FAILED"
    ErrCodeChannelFailed    ErrorCode = "CHANNEL_FAILED"
    ErrCodePluginFailed     ErrorCode = "PLUGIN_FAILED"
    ErrCodeConfigInvalid    ErrorCode = "CONFIG_INVALID"
    ErrCodeRateLimited      ErrorCode = "RATE_LIMITED"
    ErrCodeTimeout          ErrorCode = "TIMEOUT"
)

type AppError struct {
    Code    ErrorCode
    Message string
    Cause   error
}

func (e *AppError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}
```

### 8.2 Retry Logic

```go
// internal/core/retry.go

type RetryConfig struct {
    MaxRetries  int
    BaseDelay   time.Duration
    MaxDelay    time.Duration
    RetryableFn func(error) bool
}

func WithRetry(ctx context.Context, config RetryConfig, fn func() error) error {
    var lastErr error

    for i := 0; i <= config.MaxRetries; i++ {
        if err := fn(); err != nil {
            lastErr = err

            if !config.RetryableFn(err) {
                return err
            }

            if i < config.MaxRetries {
                delay := calculateBackoff(i, config)
                select {
                case <-ctx.Done():
                    return ctx.Err()
                case <-time.After(delay):
                    continue
                }
            }
        } else {
            return nil
        }
    }

    return lastErr
}
```

### 8.3 Error Handling Strategy

| Error Type | Strategy |
|------------|----------|
| Rate Limited (429) | Exponential backoff, max 3 retries |
| Server Error (5xx) | Exponential backoff, max 3 retries |
| Timeout | Retry with longer timeout |
| Invalid Request (4xx) | Log error, notify user |
| Network Error | Retry with backoff |
| Plugin Error | Log error, attempt recovery |

## 9. Testing Strategy

### 9.1 Unit Tests

**Core Components:**
- SessionManager: CRUD operations, history trimming, cleanup
- EventBus: Publish/Subscribe, handler execution
- ConfigManager: YAML parsing, env var substitution

**Plugin System:**
- PluginManager: Load/Unload, type resolution
- Registry: Factory registration, plugin creation

**Channel Plugins:**
- FeishuPlugin: Message handling, deduplication, session ID generation
- StreamingManager: Throttle logic, card updates

**Provider Plugins:**
- OpenAIProvider: API calls, response parsing, error handling
- StreamParser: SSE parsing, chunk handling

### 9.2 Integration Tests

- Full message flow: Feishu → Engine → Provider → Response
- Session persistence across multiple messages
- Plugin lifecycle: Load → Start → Stop → Unload
- Error recovery and retry logic

### 9.3 Test Coverage Targets

| Component | Target Coverage |
|-----------|----------------|
| Core Engine | 90%+ |
| Session Manager | 95%+ |
| Event Bus | 95%+ |
| Plugin System | 90%+ |
| Channel Plugins | 85%+ |
| Provider Plugins | 85%+ |

### 9.4 Test File Structure

```
internal/
├── core/
│   ├── engine_test.go
│   ├── session_test.go
│   └── event_test.go
├── config/
│   └── config_test.go
└── plugin/
    ├── manager_test.go
    └── registry_test.go

plugins/
├── channels/
│   └── feishu/
│       ├── plugin_test.go
│       ├── handler_test.go
│       └── streaming_test.go
└── providers/
    ├── openai/
    │   ├── provider_test.go
    │   └── streaming_test.go
    └── claude/
        └── provider_test.go
```

## 10. Implementation Phases

### Phase 1: Core Foundation
- Project structure setup (go.mod, directories)
- Core interfaces definition
- Engine skeleton
- SessionManager implementation
- EventBus implementation
- ConfigManager implementation

### Phase 2: Plugin System
- Plugin interfaces
- PluginManager implementation
- Plugin Registry
- Plugin lifecycle management

### Phase 3: Provider Plugins
- OpenAI provider (chat + streaming)
- Claude provider (chat + streaming)
- Provider abstraction layer

### Phase 4: Channel Plugins
- Feishu plugin structure
- Message handling
- Streaming output (Card Kit)
- Deduplication

### Phase 5: Integration & Testing
- Wire all components together
- Integration tests
- End-to-end testing
- Documentation

### Phase 6: Polish & Deploy
- Error handling refinement
- Logging improvements
- Configuration validation
- Deployment guide

## 11. Configuration Reference

### 11.1 Complete Configuration Example

```yaml
# golem.yaml

server:
  host: "0.0.0.0"
  port: 9921

llm:
  default_provider: "openai"
  providers:
    openai:
      api_key: "${OPENAI_API_KEY}"
      base_url: "https://api.openai.com/v1"
      model: "gpt-4o"
      temperature: 0.7
      max_tokens: 4096
    claude:
      api_key: "${ANTHROPIC_API_KEY}"
      base_url: "https://api.anthropic.com"
      model: "claude-3-opus-20240229"
      temperature: 0.7
      max_tokens: 4096
    gemini:
      api_key: "${GOOGLE_API_KEY}"
      base_url: "https://generativelanguage.googleapis.com"
      model: "gemini-pro"
      temperature: 0.7
      max_tokens: 4096

session:
  max_history: 50
  trim_to: 20
  idle_timeout: 30m
  cleanup_interval: 5m

plugins:
  channels:
    feishu:
      app_id: "${FEISHU_APP_ID}"
      app_secret: "${FEISHU_APP_SECRET}"
      verification_token: "${FEISHU_VERIFICATION_TOKEN}"
      encrypt_key: "${FEISHU_ENCRYPT_KEY}"

logging:
  level: "info"  # debug, info, warn, error
  format: "json" # json, text
```

### 11.2 Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `OPENAI_API_KEY` | OpenAI API key | Yes (if using OpenAI) |
| `ANTHROPIC_API_KEY` | Anthropic API key | Yes (if using Claude) |
| `GOOGLE_API_KEY` | Google API key | Yes (if using Gemini) |
| `FEISHU_APP_ID` | Feishu app ID | Yes (if using Feishu) |
| `FEISHU_APP_SECRET` | Feishu app secret | Yes (if using Feishu) |
| `FEISHU_VERIFICATION_TOKEN` | Feishu verification token | Yes (if using Feishu) |
| `FEISHU_ENCRYPT_KEY` | Feishu encrypt key | No |

## 12. Deployment

### 12.1 Build

```bash
# Build for current platform
go build -o golem ./cmd/golem

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o golem-linux ./cmd/golem

# Build for macOS
GOOS=darwin GOARCH=arm64 go build -o golem-mac ./cmd/golem
```

### 12.2 Run

```bash
# Run with default config
./golem

# Run with custom config
./golem -config /path/to/golem.yaml

# Run with environment variables
OPENAI_API_KEY=sk-xxx FEISHU_APP_ID=xxx ./golem
```

### 12.3 Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o golem ./cmd/golem

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/golem .
COPY --from=builder /app/configs/golem.example.yaml ./golem.yaml
CMD ["./golem"]
```

## 13. Future Extensions

### 13.1 Planned Features

1. **WeChat Plugin**: WeChat Official Account integration
2. **Telegram Plugin**: Telegram Bot API integration
3. **Tool Plugins**: Web search, file operations, code execution
4. **Multi-modal Support**: Image understanding, voice input/output
5. **Database Persistence**: SQLite/PostgreSQL for session storage
6. **Web UI**: Browser-based chat interface
7. **API Server**: REST API for external integrations

### 13.2 Extension Points

- New channel plugins: Implement `ChannelPlugin` interface
- New provider plugins: Implement `ProviderPlugin` interface
- New tool plugins: Implement `ToolPlugin` interface
- Custom event handlers: Subscribe to `EventBus` events

## Appendix A: Glossary

| Term | Definition |
|------|-----------|
| Channel Plugin | Plugin that handles messaging platform integration |
| Provider Plugin | Plugin that handles LLM API integration |
| Tool Plugin | Plugin that provides additional capabilities |
| Session | A conversation context between user and AI |
| StreamChunk | A piece of streaming response content |
| EventBus | Pub/sub system for component communication |
| Deduplication | Preventing duplicate message processing |

## Appendix B: References

- [Feishu Open Platform](https://open.feishu.cn)
- [oapi-sdk-go](https://github.com/larksuite/oapi-sdk-go)
- [OpenAI API Documentation](https://platform.openai.com/docs)
- [Anthropic API Documentation](https://docs.anthropic.com)
- [Google AI Documentation](https://ai.google.dev)
