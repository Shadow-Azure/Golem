# AI Agent + Feishu Bot Design Spec

> **Date**: 2026-06-06
> **Status**: Draft
> **Scope**: Basic AI chat agent with Feishu bot integration

---

## 1. Overview

Build a minimal AI agent that supports multi-turn chat via a Feishu (Lark) bot. The system uses a hybrid architecture: Rust daemon for core AI logic + TypeScript gateway for Feishu integration.

**Goals**:
- Multi-turn conversation with context persistence
- Group chat (@bot trigger) and private chat support
- Streaming response via Feishu Card Kit API
- Pluggable LLM provider abstraction (initial: OpenAI-compatible)

**Non-goals** (for this iteration):
- Tool invocation / function calling
- Persistent storage (database)
- Multi-channel support (only Feishu)
- Authentication / access control
- Electron GUI integration

---

## 2. Architecture

```
+---------------+    WebSocket     +-------------------+    HTTPS    +--------------+
| Feishu Server | <-------------> | feishu-gateway     |             | LLM Provider |
|               |                  | (TypeScript)       |             | (OpenAI/     |
+---------------+                  | - Feishu WS conn   |             |  Claude/...) |
                                   | - Message routing  |             +------+-------+
                                   | - Card Kit stream  |                    |
                                   +--------+----------+                    | HTTPS
                                            | WebSocket                     |
                                   +--------v----------+                    |
                                   | golem-daemon       | <-----------------+
                                   | (Rust)             |
                                   | - Session mgmt     |
                                   | - LLM calls        |
                                   | - Stream responses |
                                   +-------------------+
```

### 2.1 Component Responsibilities

| Component | Language | Responsibility |
|-----------|----------|---------------|
| **golem-daemon** | Rust | WebSocket server, session management, LLM provider calls, streaming response generation |
| **feishu-gateway** | TypeScript | Feishu SDK WebSocket connection, message format conversion, Card Kit streaming output |
| **golem-core** | Rust | Shared types (Session, Message, Task), LLM provider trait |

### 2.2 Process Communication

- feishu-gateway connects to golem-daemon via WebSocket (`ws://localhost:9921/ws`)
- JSON-based request/response protocol
- Supports streaming: daemon pushes `chat_delta` events as LLM generates tokens

---

## 3. Internal Protocol (WebSocket JSON)

### 3.1 Gateway -> Daemon

**Chat Request**:
```json
{
  "type": "chat_request",
  "session_id": "sess_xxx",
  "user_id": "ou_xxx",
  "content": "Hello, how are you?",
  "channel": "feishu"
}
```

### 3.2 Daemon -> Gateway

**Stream Delta** (sent for each token):
```json
{
  "type": "chat_delta",
  "session_id": "sess_xxx",
  "delta": "I'm"
}
```

**Stream Done** (sent when complete):
```json
{
  "type": "chat_done",
  "session_id": "sess_xxx",
  "full_response": "I'm doing well, thanks! How can I help you?"
}
```

**Error**:
```json
{
  "type": "chat_error",
  "session_id": "sess_xxx",
  "error": "LLM API timeout"
}
```

### 3.3 Session ID Convention

- Private chat: `feishu:{user_id}` (e.g., `feishu:ou_abc123`)
- Group chat: `feishu:group:{chat_id}:{user_id}` (per-user context in group)

---

## 4. Rust Daemon (golem-daemon)

### 4.1 Dependencies (new)

```toml
axum = { version = "0.7", features = ["ws"] }
tokio-tungstenite = "0.24"
reqwest = { version = "0.12", features = ["json", "stream"] }
async-trait = "0.1"
futures = "0.3"
toml = "0.8"
```

### 4.2 Modules

| Module | Purpose |
|--------|---------|
| `main.rs` | Entry point: load config, start WebSocket server |
| `config.rs` | Load `golem.toml`, parse into `AppConfig` struct |
| `ws.rs` | WebSocket connection handler, protocol parsing, response dispatch |
| `chat.rs` | Session management, LLM call orchestration, streaming assembly |

### 4.3 WebSocket Server

- Bind to `0.0.0.0:9921` (configurable)
- Accept WebSocket upgrades at `/ws`
- Each connection is handled in a spawned tokio task
- Connection state: session map reference + LLM provider reference

### 4.4 Session Management

- `HashMap<String, Session>` keyed by session_id
- Each `Session` holds a `Vec<Message>` (conversation history)
- Auto-trim: keep last 20 messages when history exceeds 50 messages
- Session idle timeout: 30 minutes (configurable)
- Background cleanup task: runs every 5 minutes, removes sessions idle beyond timeout

### 4.5 Graceful Shutdown

- SIGINT/SIGTERM triggers shutdown sequence
- Stop accepting new WebSocket connections
- Wait for in-flight LLM calls to complete (timeout: 10s)
- Close all WebSocket connections with close frame
- Flush logs

### 4.6 LLM Provider Abstraction

```rust
#[async_trait]
pub trait LlmProvider: Send + Sync {
    fn name(&self) -> &str;

    async fn chat(&self, messages: &[Message], config: &ChatConfig) -> Result<String>;

    async fn chat_stream(
        &self,
        messages: &[Message],
        config: &ChatConfig,
    ) -> Result<Pin<Box<dyn Stream<Item = Result<ChatDelta>> + Send>>>;
}

pub struct ChatConfig {
    pub model: String,
    pub system_prompt: Option<String>,
    pub temperature: Option<f32>,
    pub max_tokens: Option<u32>,
}

pub struct ChatDelta {
    pub content: Option<String>,
    pub finish_reason: Option<String>,
}
```

### 4.7 OpenAI Provider Implementation

- Endpoint: `{base_url}/v1/chat/completions`
- Non-streaming: parse `choices[0].message.content`
- Streaming: `stream: true`, parse SSE `data: {...}` lines, yield `ChatDelta`
- Error handling: timeout (30s), rate limit (429), invalid response
- Compatible with OpenAI-compatible APIs (DeepSeek, local models via base_url)

---

## 5. TypeScript Feishu Gateway (feishu-gateway)

### 5.1 Dependencies

```json
{
  "@larksuiteoapi/node-sdk": "^1.40.0",
  "ws": "^8.18.0",
  "dotenv": "^16.4.0",
  "typescript": "^5.7.0"
}
```

### 5.2 Modules

| Module | Purpose |
|--------|---------|
| `index.ts` | Entry: load env, connect to Feishu + daemon, handle shutdown |
| `feishu.ts` | Feishu WSClient setup, message event listener, send/reply functions |
| `daemon-client.ts` | WebSocket client to daemon, request/response matching, reconnect logic |
| `streaming.ts` | Card Kit streaming: create card, update content, close stream |
| `handler.ts` | Message routing: parse Feishu event -> build daemon request -> dispatch response |
| `types.ts` | Protocol type definitions matching daemon JSON schema |

### 5.3 Feishu Connection

- Use `@larksuiteoapi/node-sdk` `WSClient` for WebSocket long connection
- Listen to `im.message.receive_v1` event
- Message deduplication: track recent message IDs (LRU cache, 5min TTL)
- Group chat: only respond when bot is mentioned (@)
- Private chat: always respond

### 5.4 Card Kit Streaming

Flow:
1. Receive first `chat_delta` from daemon -> create streaming card via `POST /cardkit/v1/cards`
2. Each subsequent `chat_delta` -> throttled update via `PUT /cardkit/v1/cards/{id}/elements/content/content`
3. Receive `chat_done` -> close streaming mode via PATCH

Throttling:
- Update interval: 160ms minimum between API calls
- Significance threshold: skip update unless >= 18 new characters or sentence boundary
- Merge partial text: accumulate all deltas into full text, send snapshot (not diff)

Fallback:
- If Card Kit API fails, fall back to regular message reply (wait for full response)
- 60-second backoff before retrying Card Kit after failure

### 5.5 Configuration

```env
# Feishu
FEISHU_APP_ID=cli_xxx
FEISHU_APP_SECRET=xxx
FEISHU_VERIFICATION_TOKEN=xxx

# Daemon
DAEMON_WS_URL=ws://localhost:9921/ws
```

---

## 6. Configuration (golem.toml)

Location: `./golem.toml` (project root) or `~/.golem/config.toml` (user-level, lower priority).

```toml
[server]
port = 9921
host = "0.0.0.0"

[llm]
provider = "openai"
api_key = "sk-xxx"
model = "gpt-4o"
base_url = "https://api.openai.com/v1"  # Override for compatible APIs
system_prompt = "You are a helpful assistant."
temperature = 0.7
max_tokens = 4096

[session]
max_history = 50
trim_to = 20
idle_timeout_minutes = 30
```

---

## 7. Error Handling

| Scenario | Handling |
|----------|----------|
| LLM API timeout | Return `chat_error` to gateway, log error |
| LLM API rate limit | Retry with exponential backoff (3 attempts), then return error |
| LLM API invalid response | Return `chat_error`, log raw response |
| Feishu connection lost | Auto-reconnect with exponential backoff |
| Daemon connection lost | Auto-reconnect with exponential backoff, queue messages |
| Card Kit streaming fails | Fallback to regular message reply |
| Duplicate Feishu message | Dedup via message ID cache, ignore |
| Session not found | Create new session |

---

## 8. Testing Strategy

### 8.1 Unit Tests (Rust)

- LLM provider trait: mock HTTP responses, verify parsing
- OpenAI provider: test streaming SSE parsing, error handling
- Session management: multi-turn history, auto-trim, timeout
- Protocol types: serialization/deserialization round-trip

### 8.2 Unit Tests (TypeScript)

- Message format conversion (Feishu event -> daemon request)
- Streaming throttle logic
- Dedup logic

### 8.3 Integration Tests (Rust)

- WebSocket endpoint: send `chat_request`, verify `chat_delta` + `chat_done` sequence
- Multi-session: concurrent requests to different sessions
- Error paths: LLM failure returns `chat_error`

### 8.4 E2E Tests

- CLI or script sends message to daemon -> verifies response
- Gateway reconnects after daemon restart

### 8.5 Manual Testing

- Start daemon + gateway
- Send messages via Feishu bot (private + group)
- Verify streaming card output
- Verify multi-turn context

---

## 9. Implementation Order

1. **golem-core**: LLM provider trait + OpenAI implementation
2. **golem-daemon**: Config loading, WebSocket server, session management, LLM integration
3. **feishu-gateway**: Feishu connection, daemon client, message routing
4. **feishu-gateway**: Card Kit streaming
5. **Integration**: End-to-end testing, error handling, reconnection logic
6. **Test**: Unit tests, integration tests, manual Feishu testing
