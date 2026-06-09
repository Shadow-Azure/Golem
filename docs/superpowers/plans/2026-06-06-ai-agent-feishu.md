# AI Agent + Feishu Bot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal AI agent with multi-turn chat supporting Feishu bot integration via WebSocket streaming.

**Architecture:** Rust daemon exposes WebSocket server for chat requests. TypeScript gateway connects to Feishu SDK and proxies messages to daemon. LLM provider abstraction allows pluggable backends (initial: OpenAI-compatible). Session management with auto-trim and idle timeout.

**Tech Stack:** Rust (tokio, axum, reqwest, async-trait) · TypeScript (Feishu SDK, ws) · WebSocket JSON protocol

---

## File Structure

### golem-core (shared types + LLM trait)

| File | Purpose |
|------|---------|
| `crates/golem-core/src/lib.rs` | Re-export new modules |
| `crates/golem-core/src/llm.rs` | LlmProvider trait, ChatConfig, ChatDelta |
| `crates/golem-core/src/protocol.rs` | WebSocket protocol types (ChatRequest, StreamDelta, StreamDone, ChatError) |
| `crates/golem-core/src/session.rs` | **Modify** — add `last_active_at`, auto-trim, idle check |
| `crates/golem-core/Cargo.toml` | **Modify** — add async-trait, tokio (time feature) |

### golem-daemon (WebSocket server + LLM integration)

| File | Purpose |
|------|---------|
| `crates/golem-daemon/src/main.rs` | **Modify** — load config, start server, graceful shutdown |
| `crates/golem-daemon/src/config.rs` | Load `golem.toml` into AppConfig |
| `crates/golem-daemon/src/ws.rs` | axum WebSocket handler, protocol dispatch |
| `crates/golem-daemon/src/chat.rs` | Session manager, LLM call orchestration, streaming |
| `crates/golem-daemon/src/openai.rs` | OpenAI-compatible LLM provider implementation |
| `crates/golem-daemon/Cargo.toml` | **Modify** — add axum, reqwest, async-trait, futures, toml |

### feishu-gateway (TypeScript)

| File | Purpose |
|------|---------|
| `feishu-gateway/package.json` | Dependencies |
| `feishu-gateway/tsconfig.json` | TypeScript config |
| `feishu-gateway/src/index.ts` | Entry: load env, connect Feishu + daemon |
| `feishu-gateway/src/feishu.ts` | Feishu WSClient, message listener |
| `feishu-gateway/src/daemon-client.ts` | WebSocket client to daemon |
| `feishu-gateway/src/streaming.ts` | Card Kit streaming logic |
| `feishu-gateway/src/handler.ts` | Message routing: Feishu event -> daemon request |
| `feishu-gateway/src/types.ts` | Protocol type definitions |

---

## Task 1: LLM Provider Trait + Types (golem-core)

**Files:**
- Modify: `crates/golem-core/Cargo.toml`
- Create: `crates/golem-core/src/llm.rs`
- Modify: `crates/golem-core/src/lib.rs`

- [ ] **Step 1: Add async-trait dependency**

Edit `crates/golem-core/Cargo.toml`, add under `[dependencies]`:

```toml
async-trait = "0.1"
tokio = { workspace = true, features = ["sync"] }
```

- [ ] **Step 2: Write failing tests for ChatConfig and ChatDelta**

Create `crates/golem-core/src/llm.rs`:

```rust
use serde::{Deserialize, Serialize};

/// Configuration for an LLM chat call.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatConfig {
    pub model: String,
    pub system_prompt: Option<String>,
    pub temperature: Option<f32>,
    pub max_tokens: Option<u32>,
}

/// A single streaming delta from the LLM.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatDelta {
    pub content: Option<String>,
    pub finish_reason: Option<String>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_chat_config_serialization() {
        let config = ChatConfig {
            model: "gpt-4o".to_string(),
            system_prompt: Some("You are helpful.".to_string()),
            temperature: Some(0.7),
            max_tokens: Some(4096),
        };
        let json = serde_json::to_string(&config).unwrap();
        let deserialized: ChatConfig = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.model, "gpt-4o");
        assert_eq!(deserialized.temperature, Some(0.7));
    }

    #[test]
    fn test_chat_delta_content() {
        let delta = ChatDelta {
            content: Some("Hello".to_string()),
            finish_reason: None,
        };
        assert_eq!(delta.content.as_deref(), Some("Hello"));
        assert!(delta.finish_reason.is_none());
    }

    #[test]
    fn test_chat_delta_finish() {
        let delta = ChatDelta {
            content: None,
            finish_reason: Some("stop".to_string()),
        };
        assert!(delta.content.is_none());
        assert_eq!(delta.finish_reason.as_deref(), Some("stop"));
    }
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cargo test -p golem-core --lib llm`
Expected: FAIL — `llm` module not declared in lib.rs

- [ ] **Step 4: Add Message conversion helpers and LlmProvider trait**

Append to `crates/golem-core/src/llm.rs` (after the existing structs, before `#[cfg(test)]`):

```rust
use async_trait::async_trait;
use futures::Stream;
use std::pin::Pin;

use crate::session::Message;

/// Convert golem-core Message to OpenAI-format message dict.
pub fn messages_to_openai(messages: &[Message], system_prompt: Option<&str>) -> Vec<serde_json::Value> {
    let mut result = Vec::new();
    if let Some(prompt) = system_prompt {
        result.push(serde_json::json!({
            "role": "system",
            "content": prompt,
        }));
    }
    for msg in messages {
        let role = match msg.role {
            crate::session::MessageRole::User => "user",
            crate::session::MessageRole::Assistant => "assistant",
            crate::session::MessageRole::System => "system",
        };
        result.push(serde_json::json!({
            "role": role,
            "content": msg.content,
        }));
    }
    result
}

/// Trait for LLM providers (OpenAI-compatible API).
#[async_trait]
pub trait LlmProvider: Send + Sync {
    fn name(&self) -> &str;

    /// Non-streaming chat completion.
    async fn chat(&self, messages: &[Message], config: &ChatConfig) -> anyhow::Result<String>;

    /// Streaming chat completion.
    async fn chat_stream(
        &self,
        messages: &[Message],
        config: &ChatConfig,
    ) -> anyhow::Result<Pin<Box<dyn Stream<Item = anyhow::Result<ChatDelta>> + Send>>>;
}
```

Also add `futures` to `crates/golem-core/Cargo.toml`:

```toml
futures = "0.3"
```

- [ ] **Step 5: Declare module in lib.rs**

Edit `crates/golem-core/src/lib.rs`:

```rust
//! Golem core library
//!
//! Task-oriented AI assistant framework core types and logic.

pub mod llm;
pub mod memory;
pub mod session;
pub mod task;

pub use llm::{ChatConfig, ChatDelta, LlmProvider};
pub use session::Session;
pub use task::Task;
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cargo test -p golem-core --lib llm`
Expected: 3 tests PASS

- [ ] **Step 7: Run clippy + fmt**

Run: `cargo clippy -p golem-core --all-targets -- -D warnings && cargo fmt -p golem-core -- --check`
Expected: clean

- [ ] **Step 8: Commit**

```bash
git add crates/golem-core/Cargo.toml crates/golem-core/src/llm.rs crates/golem-core/src/lib.rs
git commit -m "feat(core): add LLM provider trait, ChatConfig, and ChatDelta types"
```

---

## Task 2: Protocol Types (golem-core)

**Files:**
- Create: `crates/golem-core/src/protocol.rs`

- [ ] **Step 1: Write failing tests for protocol types**

Create `crates/golem-core/src/protocol.rs`:

```rust
use serde::{Deserialize, Serialize};

/// Gateway -> Daemon: request a chat completion.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatRequest {
    #[serde(rename = "type")]
    pub msg_type: String,
    pub session_id: String,
    pub user_id: String,
    pub content: String,
    pub channel: String,
}

/// Daemon -> Gateway: streaming token delta.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StreamDelta {
    #[serde(rename = "type")]
    pub msg_type: String,
    pub session_id: String,
    pub delta: String,
}

/// Daemon -> Gateway: streaming complete.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StreamDone {
    #[serde(rename = "type")]
    pub msg_type: String,
    pub session_id: String,
    pub full_response: String,
}

/// Daemon -> Gateway: error occurred.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatErrorResponse {
    #[serde(rename = "type")]
    pub msg_type: String,
    pub session_id: String,
    pub error: String,
}

/// Discriminated union for all daemon -> gateway messages.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum DaemonMessage {
    #[serde(rename = "chat_delta")]
    StreamDelta(StreamDelta),
    #[serde(rename = "chat_done")]
    StreamDone(StreamDone),
    #[serde(rename = "chat_error")]
    ChatError(ChatErrorResponse),
}

impl ChatRequest {
    pub fn new(
        session_id: impl Into<String>,
        user_id: impl Into<String>,
        content: impl Into<String>,
        channel: impl Into<String>,
    ) -> Self {
        Self {
            msg_type: "chat_request".to_string(),
            session_id: session_id.into(),
            user_id: user_id.into(),
            content: content.into(),
            channel: channel.into(),
        }
    }
}

impl StreamDelta {
    pub fn new(session_id: impl Into<String>, delta: impl Into<String>) -> Self {
        Self {
            msg_type: "chat_delta".to_string(),
            session_id: session_id.into(),
            delta: delta.into(),
        }
    }
}

impl StreamDone {
    pub fn new(session_id: impl Into<String>, full_response: impl Into<String>) -> Self {
        Self {
            msg_type: "chat_done".to_string(),
            session_id: session_id.into(),
            full_response: full_response.into(),
        }
    }
}

impl ChatErrorResponse {
    pub fn new(session_id: impl Into<String>, error: impl Into<String>) -> Self {
        Self {
            msg_type: "chat_error".to_string(),
            session_id: session_id.into(),
            error: error.into(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_chat_request_roundtrip() {
        let req = ChatRequest::new("sess_1", "ou_abc", "hello", "feishu");
        let json = serde_json::to_string(&req).unwrap();
        assert!(json.contains("\"type\":\"chat_request\""));
        let deserialized: ChatRequest = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.session_id, "sess_1");
        assert_eq!(deserialized.content, "hello");
    }

    #[test]
    fn test_stream_delta_roundtrip() {
        let delta = StreamDelta::new("sess_1", "token");
        let json = serde_json::to_string(&delta).unwrap();
        let deserialized: StreamDelta = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.delta, "token");
    }

    #[test]
    fn test_stream_done_roundtrip() {
        let done = StreamDone::new("sess_1", "full text");
        let json = serde_json::to_string(&done).unwrap();
        assert!(json.contains("\"type\":\"chat_done\""));
        let deserialized: StreamDone = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.full_response, "full text");
    }

    #[test]
    fn test_chat_error_roundtrip() {
        let err = ChatErrorResponse::new("sess_1", "timeout");
        let json = serde_json::to_string(&err).unwrap();
        assert!(json.contains("\"type\":\"chat_error\""));
        let deserialized: ChatErrorResponse = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.error, "timeout");
    }

    #[test]
    fn test_daemon_message_tagged_enum() {
        let delta = StreamDelta::new("sess_1", "hi");
        let msg = DaemonMessage::StreamDelta(delta);
        let json = serde_json::to_string(&msg).unwrap();
        assert!(json.contains("\"type\":\"chat_delta\""));
        let deserialized: DaemonMessage = serde_json::from_str(&json).unwrap();
        match deserialized {
            DaemonMessage::StreamDelta(d) => assert_eq!(d.delta, "hi"),
            _ => panic!("wrong variant"),
        }
    }
}
```

- [ ] **Step 2: Declare protocol module in lib.rs**

Edit `crates/golem-core/src/lib.rs`, add `pub mod protocol;`:

```rust
//! Golem core library
//!
//! Task-oriented AI assistant framework core types and logic.

pub mod llm;
pub mod memory;
pub mod protocol;
pub mod session;
pub mod task;

pub use llm::{ChatConfig, ChatDelta, LlmProvider};
pub use session::Session;
pub use task::Task;
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cargo test -p golem-core --lib protocol`
Expected: 5 tests PASS

- [ ] **Step 4: Run clippy + fmt**

Run: `cargo clippy -p golem-core --all-targets -- -D warnings && cargo fmt -p golem-core -- --check`
Expected: clean

- [ ] **Step 5: Commit**

```bash
git add crates/golem-core/src/protocol.rs
git commit -m "feat(core): add WebSocket protocol types for daemon-gateway communication"
```

---

## Task 3: Update Session Type (golem-core)

**Files:**
- Modify: `crates/golem-core/src/session.rs`

- [ ] **Step 1: Write failing tests for auto-trim and idle check**

Add these tests to the `#[cfg(test)] mod tests` block in `crates/golem-core/src/session.rs`:

```rust
    #[test]
    fn test_auto_trim_under_threshold() {
        let mut session = Session::new("s1", "t1");
        for i in 0..49 {
            session.add_message(MessageRole::User, format!("msg {i}"));
        }
        session.trim_history(50, 20);
        assert_eq!(session.messages.len(), 49);
    }

    #[test]
    fn test_auto_trim_over_threshold() {
        let mut session = Session::new("s1", "t1");
        for i in 0..60 {
            session.add_message(MessageRole::User, format!("msg {i}"));
        }
        session.trim_history(50, 20);
        // Should trim to last 20 messages
        assert_eq!(session.messages.len(), 20);
        // First message should be "msg 40"
        assert_eq!(session.messages[0].content, "msg 40");
    }

    #[test]
    fn test_session_idle_timeout() {
        use std::time::{Duration, Instant};
        let mut session = Session::new("s1", "t1");
        // Simulate idle session by setting last_active_at to past
        session.last_active_at = Instant::now() - Duration::from_secs(31 * 60);
        assert!(session.is_idle(Duration::from_secs(30 * 60)));
    }

    #[test]
    fn test_session_not_idle() {
        use std::time::Duration;
        let mut session = Session::new("s1", "t1");
        session.touch();
        assert!(!session.is_idle(Duration::from_secs(30 * 60)));
    }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cargo test -p golem-core --lib session`
Expected: FAIL — `trim_history`, `last_active_at`, `is_idle`, `touch` not found

- [ ] **Step 3: Implement auto-trim and idle timeout**

Replace the full content of `crates/golem-core/src/session.rs`:

```rust
use serde::{Deserialize, Serialize};
use std::time::{Duration, Instant};

/// Represents a conversation session.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    pub id: String,
    pub task_id: Option<String>,
    pub messages: Vec<Message>,
    #[serde(skip)]
    pub last_active_at: Instant,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    pub role: MessageRole,
    pub content: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum MessageRole {
    User,
    Assistant,
    System,
}

impl Session {
    pub fn new(id: impl Into<String>, task_id: Option<String>) -> Self {
        Self {
            id: id.into(),
            task_id,
            messages: Vec::new(),
            last_active_at: Instant::now(),
        }
    }

    pub fn add_message(&mut self, role: MessageRole, content: impl Into<String>) {
        self.messages.push(Message {
            role,
            content: content.into(),
        });
        self.touch();
    }

    /// Update last_active_at to now.
    pub fn touch(&mut self) {
        self.last_active_at = Instant::now();
    }

    /// Trim message history when it exceeds `max_history`, keeping last `keep` messages.
    pub fn trim_history(&mut self, max_history: usize, keep: usize) {
        if self.messages.len() > max_history {
            let drain_count = self.messages.len() - keep;
            self.messages.drain(..drain_count);
        }
    }

    /// Check if session has been idle beyond the given duration.
    pub fn is_idle(&self, timeout: Duration) -> bool {
        self.last_active_at.elapsed() > timeout
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_session_creation() {
        let session = Session::new("s1", Some("t1".to_string()));
        assert!(session.messages.is_empty());
    }

    #[test]
    fn test_add_message() {
        let mut session = Session::new("s1", Some("t1".to_string()));
        session.add_message(MessageRole::User, "hello");
        assert_eq!(session.messages.len(), 1);
    }

    #[test]
    fn test_auto_trim_under_threshold() {
        let mut session = Session::new("s1", Some("t1".to_string()));
        for i in 0..49 {
            session.add_message(MessageRole::User, format!("msg {i}"));
        }
        session.trim_history(50, 20);
        assert_eq!(session.messages.len(), 49);
    }

    #[test]
    fn test_auto_trim_over_threshold() {
        let mut session = Session::new("s1", Some("t1".to_string()));
        for i in 0..60 {
            session.add_message(MessageRole::User, format!("msg {i}"));
        }
        session.trim_history(50, 20);
        assert_eq!(session.messages.len(), 20);
        assert_eq!(session.messages[0].content, "msg 40");
    }

    #[test]
    fn test_session_idle_timeout() {
        use std::time::Duration;
        let mut session = Session::new("s1", Some("t1".to_string()));
        session.last_active_at = Instant::now() - Duration::from_secs(31 * 60);
        assert!(session.is_idle(Duration::from_secs(30 * 60)));
    }

    #[test]
    fn test_session_not_idle() {
        use std::time::Duration;
        let mut session = Session::new("s1", Some("t1".to_string()));
        session.touch();
        assert!(!session.is_idle(Duration::from_secs(30 * 60)));
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cargo test -p golem-core --lib session`
Expected: 6 tests PASS

- [ ] **Step 5: Verify existing tests still pass**

Run: `cargo test -p golem-core`
Expected: All tests PASS (session + llm + protocol + memory + task)

- [ ] **Step 6: Run clippy + fmt**

Run: `cargo clippy -p golem-core --all-targets -- -D warnings && cargo fmt -p golem-core -- --check`
Expected: clean

- [ ] **Step 7: Commit**

```bash
git add crates/golem-core/src/session.rs
git commit -m "feat(core): add session auto-trim and idle timeout support"
```

---

## Task 4: Config Module (golem-daemon)

**Files:**
- Modify: `crates/golem-daemon/Cargo.toml`
- Create: `crates/golem-daemon/src/config.rs`
- Modify: `crates/golem-daemon/src/main.rs`

- [ ] **Step 1: Add dependencies**

Edit `crates/golem-daemon/Cargo.toml`:

```toml
[package]
name = "golem-daemon"
version.workspace = true
edition.workspace = true
license.workspace = true

[[bin]]
name = "golem-daemon"
path = "src/main.rs"

[dependencies]
golem-core.workspace = true
tokio.workspace = true
serde.workspace = true
serde_json.workspace = true
anyhow.workspace = true
tracing.workspace = true
tracing-subscriber.workspace = true
axum = { version = "0.7", features = ["ws"] }
reqwest = { version = "0.12", features = ["json", "stream"] }
async-trait = "0.1"
futures = "0.3"
toml = "0.8"
```

- [ ] **Step 2: Write failing tests for config parsing**

Create `crates/golem-daemon/src/config.rs`:

```rust
use serde::Deserialize;
use std::path::Path;

/// Top-level application config loaded from golem.toml.
#[derive(Debug, Clone, Deserialize)]
pub struct AppConfig {
    pub server: ServerConfig,
    pub llm: LlmConfig,
    #[serde(default)]
    pub session: SessionConfig,
}

#[derive(Debug, Clone, Deserialize)]
pub struct ServerConfig {
    #[serde(default = "default_port")]
    pub port: u16,
    #[serde(default = "default_host")]
    pub host: String,
}

#[derive(Debug, Clone, Deserialize)]
pub struct LlmConfig {
    #[serde(default = "default_provider")]
    pub provider: String,
    pub api_key: String,
    #[serde(default = "default_model")]
    pub model: String,
    #[serde(default = "default_base_url")]
    pub base_url: String,
    #[serde(default)]
    pub system_prompt: Option<String>,
    #[serde(default = "default_temperature")]
    pub temperature: f32,
    #[serde(default = "default_max_tokens")]
    pub max_tokens: u32,
}

#[derive(Debug, Clone, Deserialize)]
pub struct SessionConfig {
    #[serde(default = "default_max_history")]
    pub max_history: usize,
    #[serde(default = "default_trim_to")]
    pub trim_to: usize,
    #[serde(default = "default_idle_timeout_minutes")]
    pub idle_timeout_minutes: u64,
}

impl Default for SessionConfig {
    fn default() -> Self {
        Self {
            max_history: default_max_history(),
            trim_to: default_trim_to(),
            idle_timeout_minutes: default_idle_timeout_minutes(),
        }
    }
}

fn default_port() -> u16 {
    9921
}
fn default_host() -> String {
    "0.0.0.0".to_string()
}
fn default_provider() -> String {
    "openai".to_string()
}
fn default_model() -> String {
    "gpt-4o".to_string()
}
fn default_base_url() -> String {
    "https://api.openai.com/v1".to_string()
}
fn default_temperature() -> f32 {
    0.7
}
fn default_max_tokens() -> u32 {
    4096
}
fn default_max_history() -> usize {
    50
}
fn default_trim_to() -> usize {
    20
}
fn default_idle_timeout_minutes() -> u64 {
    30
}

impl AppConfig {
    /// Load config from a TOML file. Falls back to defaults for missing fields.
    pub fn from_file(path: &Path) -> anyhow::Result<Self> {
        let content = std::fs::read_to_string(path)?;
        let config: AppConfig = toml::from_str(&content)?;
        Ok(config)
    }

    /// Load config with priority: ./golem.toml > ~/.golem/config.toml > defaults.
    pub fn load() -> anyhow::Result<Self> {
        let local = Path::new("golem.toml");
        if local.exists() {
            return Self::from_file(local);
        }

        if let Some(home) = dirs::home_dir() {
            let user_config = home.join(".golem").join("config.toml");
            if user_config.exists() {
                return Self::from_file(&user_config);
            }
        }

        anyhow::bail!("No config file found. Create golem.toml or ~/.golem/config.toml")
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;

    #[test]
    fn test_parse_full_config() {
        let toml = r#"
[server]
port = 8080
host = "127.0.0.1"

[llm]
provider = "openai"
api_key = "sk-test"
model = "gpt-4o"
base_url = "https://api.openai.com/v1"
system_prompt = "You are helpful."
temperature = 0.5
max_tokens = 2048

[session]
max_history = 100
trim_to = 30
idle_timeout_minutes = 60
"#;
        let config: AppConfig = toml::from_str(toml).unwrap();
        assert_eq!(config.server.port, 8080);
        assert_eq!(config.server.host, "127.0.0.1");
        assert_eq!(config.llm.api_key, "sk-test");
        assert_eq!(config.llm.temperature, 0.5);
        assert_eq!(config.session.max_history, 100);
    }

    #[test]
    fn test_parse_minimal_config() {
        let toml = r#"
[llm]
api_key = "sk-test"
"#;
        let config: AppConfig = toml::from_str(toml).unwrap();
        assert_eq!(config.server.port, 9921);
        assert_eq!(config.llm.model, "gpt-4o");
        assert_eq!(config.llm.base_url, "https://api.openai.com/v1");
        assert_eq!(config.session.max_history, 50);
    }

    #[test]
    fn test_from_file() {
        let dir = std::env::temp_dir().join("golem_test_config");
        std::fs::create_dir_all(&dir).unwrap();
        let path = dir.join("golem.toml");
        let mut f = std::fs::File::create(&path).unwrap();
        writeln!(
            f,
            r#"
[llm]
api_key = "sk-file-test"
"#
        )
        .unwrap();

        let config = AppConfig::from_file(&path).unwrap();
        assert_eq!(config.llm.api_key, "sk-file-test");

        std::fs::remove_dir_all(&dir).unwrap();
    }
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cargo test -p golem-daemon --lib config`
Expected: FAIL — `config` module not declared, also `dirs` crate missing

- [ ] **Step 4: Add dirs dependency and declare module**

Add to `crates/golem-daemon/Cargo.toml` under `[dependencies]`:

```toml
dirs = "6"
```

Update `crates/golem-daemon/src/main.rs`:

```rust
use anyhow::Result;

mod config;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    tracing::info!("Golem daemon v{} starting", env!("CARGO_PKG_VERSION"));
    Ok(())
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cargo test -p golem-daemon --lib config`
Expected: 3 tests PASS

- [ ] **Step 6: Run clippy + fmt**

Run: `cargo clippy -p golem-daemon --all-targets -- -D warnings && cargo fmt -p golem-daemon -- --check`
Expected: clean

- [ ] **Step 7: Commit**

```bash
git add crates/golem-daemon/Cargo.toml crates/golem-daemon/src/config.rs crates/golem-daemon/src/main.rs
git commit -m "feat(daemon): add config module for golem.toml loading"
```

---

## Task 5: OpenAI Provider (golem-daemon)

**Files:**
- Create: `crates/golem-daemon/src/openai.rs`
- Modify: `crates/golem-daemon/src/main.rs`

- [ ] **Step 1: Write failing tests for OpenAI provider**

Create `crates/golem-daemon/src/openai.rs`:

```rust
use async_trait::async_trait;
use futures::Stream;
use golem_core::llm::{ChatConfig, ChatDelta, LlmProvider};
use golem_core::session::Message;
use reqwest::Client;
use serde::Deserialize;
use std::pin::Pin;
use std::time::Duration;

/// OpenAI-compatible LLM provider.
pub struct OpenAiProvider {
    client: Client,
    base_url: String,
    api_key: String,
}

impl OpenAiProvider {
    pub fn new(base_url: String, api_key: String) -> Self {
        let client = Client::builder()
            .timeout(Duration::from_secs(60))
            .build()
            .expect("failed to build HTTP client");
        Self {
            client,
            base_url,
            api_key,
        }
    }
}

#[derive(Deserialize)]
struct ChatCompletionResponse {
    choices: Vec<Choice>,
}

#[derive(Deserialize)]
struct Choice {
    message: ChoiceMessage,
}

#[derive(Deserialize)]
struct ChoiceMessage {
    content: String,
}

#[derive(Deserialize)]
struct StreamResponse {
    choices: Vec<StreamChoice>,
}

#[derive(Deserialize)]
struct StreamChoice {
    delta: StreamChoiceDelta,
    finish_reason: Option<String>,
}

#[derive(Deserialize)]
struct StreamChoiceDelta {
    content: Option<String>,
}

#[async_trait]
impl LlmProvider for OpenAiProvider {
    fn name(&self) -> &str {
        "openai"
    }

    async fn chat(&self, messages: &[Message], config: &ChatConfig) -> anyhow::Result<String> {
        let messages_json = golem_core::llm::messages_to_openai(messages, config.system_prompt.as_deref());

        let body = serde_json::json!({
            "model": config.model,
            "messages": messages_json,
            "temperature": config.temperature,
            "max_tokens": config.max_tokens,
            "stream": false,
        });

        let url = format!("{}/chat/completions", self.base_url);
        let resp = self
            .client
            .post(&url)
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            anyhow::bail!("LLM API error {status}: {text}");
        }

        let completion: ChatCompletionResponse = resp.json().await?;
        let content = completion
            .choices
            .first()
            .map(|c| c.message.content.clone())
            .unwrap_or_default();
        Ok(content)
    }

    async fn chat_stream(
        &self,
        messages: &[Message],
        config: &ChatConfig,
    ) -> anyhow::Result<Pin<Box<dyn Stream<Item = anyhow::Result<ChatDelta>> + Send>>> {
        use futures::TryStreamExt;
        use tokio_util::io::StreamReader;

        let messages_json = golem_core::llm::messages_to_openai(messages, config.system_prompt.as_deref());

        let body = serde_json::json!({
            "model": config.model,
            "messages": messages_json,
            "temperature": config.temperature,
            "max_tokens": config.max_tokens,
            "stream": true,
        });

        let url = format!("{}/chat/completions", self.base_url);
        let resp = self
            .client
            .post(&url)
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&body)
            .send()
            .await?;

        if !resp.status().is_success() {
            let status = resp.status();
            let text = resp.text().await.unwrap_or_default();
            anyhow::bail!("LLM API error {status}: {text}");
        }

        let byte_stream = resp.bytes_stream().map_err(|e| std::io::Error::new(std::io::ErrorKind::Other, e));
        let reader = StreamReader::new(byte_stream);
        let stream = tokio::io::BufReader::new(reader).lines();

        let output_stream = futures::stream::unfold(stream, |mut stream| async move {
            use tokio::io::AsyncBufReadExt;
            loop {
                match stream.next_line().await {
                    Ok(Some(line)) => {
                        let line = line.trim().to_string();
                        if line.is_empty() {
                            continue;
                        }
                        if line == "data: [DONE]" {
                            return Some((Ok(ChatDelta {
                                content: None,
                                finish_reason: Some("stop".to_string()),
                            }), stream));
                        }
                        if let Some(data) = line.strip_prefix("data: ") {
                            match serde_json::from_str::<StreamResponse>(data) {
                                Ok(resp) => {
                                    if let Some(choice) = resp.choices.first() {
                                        return Some((Ok(ChatDelta {
                                            content: choice.delta.content.clone(),
                                            finish_reason: choice.finish_reason.clone(),
                                        }), stream));
                                    }
                                }
                                Err(e) => {
                                    return Some((Err(anyhow::anyhow!("SSE parse error: {e}")), stream));
                                }
                            }
                        }
                    }
                    Ok(None) => return None,
                    Err(e) => return Some((Err(anyhow::anyhow!("Stream read error: {e}")), stream)),
                }
            }
        });

        Ok(Box::pin(output_stream))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_openai_provider_name() {
        let provider = OpenAiProvider::new(
            "https://api.openai.com/v1".to_string(),
            "sk-test".to_string(),
        );
        assert_eq!(provider.name(), "openai");
    }
}
```

- [ ] **Step 2: Add tokio-util dependency**

Add to `crates/golem-daemon/Cargo.toml` under `[dependencies]`:

```toml
tokio-util = { version = "0.7", features = ["io"] }
```

- [ ] **Step 3: Declare module in main.rs**

Edit `crates/golem-daemon/src/main.rs`:

```rust
use anyhow::Result;

mod config;
mod openai;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    tracing::info!("Golem daemon v{} starting", env!("CARGO_PKG_VERSION"));
    Ok(())
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cargo test -p golem-daemon --lib openai`
Expected: 1 test PASS

- [ ] **Step 5: Run clippy + fmt**

Run: `cargo clippy -p golem-daemon --all-targets -- -D warnings && cargo fmt -p golem-daemon -- --check`
Expected: clean

- [ ] **Step 6: Commit**

```bash
git add crates/golem-daemon/Cargo.toml crates/golem-daemon/src/openai.rs crates/golem-daemon/src/main.rs
git commit -m "feat(daemon): add OpenAI-compatible LLM provider with streaming"
```

---

## Task 6: Chat Session Manager (golem-daemon)

**Files:**
- Create: `crates/golem-daemon/src/chat.rs`
- Modify: `crates/golem-daemon/src/main.rs`

- [ ] **Step 1: Write failing tests for session manager**

Create `crates/golem-daemon/src/chat.rs`:

```rust
use golem_core::session::{MessageRole, Session};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::RwLock;

/// Manages chat sessions with auto-trim and idle cleanup.
pub struct SessionManager {
    sessions: Arc<RwLock<HashMap<String, Session>>>,
    max_history: usize,
    trim_to: usize,
    idle_timeout: Duration,
}

impl SessionManager {
    pub fn new(max_history: usize, trim_to: usize, idle_timeout: Duration) -> Self {
        Self {
            sessions: Arc::new(RwLock::new(HashMap::new())),
            max_history,
            trim_to,
            idle_timeout,
        }
    }

    /// Get or create a session for the given session_id.
    pub async fn get_or_create(&self, session_id: &str) -> Session {
        let mut sessions = self.sessions.write().await;
        sessions
            .entry(session_id.to_string())
            .or_insert_with(|| Session::new(session_id, None))
            .clone()
    }

    /// Add a message to a session, creating it if needed. Trims history if over threshold.
    pub async fn add_message(&self, session_id: &str, role: MessageRole, content: &str) {
        let mut sessions = self.sessions.write().await;
        let session = sessions
            .entry(session_id.to_string())
            .or_insert_with(|| Session::new(session_id, None));
        session.add_message(role, content);
        session.trim_history(self.max_history, self.trim_to);
    }

    /// Get message history for a session.
    pub async fn get_messages(&self, session_id: &str) -> Vec<golem_core::session::Message> {
        let sessions = self.sessions.read().await;
        sessions
            .get(session_id)
            .map(|s| s.messages.clone())
            .unwrap_or_default()
    }

    /// Remove idle sessions beyond timeout. Returns count of removed sessions.
    pub async fn cleanup_idle(&self) -> usize {
        let mut sessions = self.sessions.write().await;
        let before = sessions.len();
        sessions.retain(|_, session| !session.is_idle(self.idle_timeout));
        before - sessions.len()
    }

    /// Number of active sessions.
    pub async fn session_count(&self) -> usize {
        self.sessions.read().await.len()
    }
}

/// Spawn a background task that cleans up idle sessions periodically.
pub fn spawn_cleanup_task(manager: Arc<SessionManager>, interval: Duration) {
    tokio::spawn(async move {
        let mut timer = tokio::time::interval(interval);
        loop {
            timer.tick().await;
            let removed = manager.cleanup_idle().await;
            if removed > 0 {
                tracing::info!("Cleaned up {removed} idle sessions");
            }
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_get_or_create_session() {
        let manager = SessionManager::new(50, 20, Duration::from_secs(1800));
        let session = manager.get_or_create("sess_1").await;
        assert_eq!(session.id, "sess_1");
        assert!(session.messages.is_empty());
    }

    #[tokio::test]
    async fn test_add_message_creates_session() {
        let manager = SessionManager::new(50, 20, Duration::from_secs(1800));
        manager
            .add_message("sess_1", MessageRole::User, "hello")
            .await;
        let messages = manager.get_messages("sess_1").await;
        assert_eq!(messages.len(), 1);
        assert_eq!(messages[0].content, "hello");
    }

    #[tokio::test]
    async fn test_multi_turn_conversation() {
        let manager = SessionManager::new(50, 20, Duration::from_secs(1800));
        manager
            .add_message("sess_1", MessageRole::User, "hi")
            .await;
        manager
            .add_message("sess_1", MessageRole::Assistant, "hello!")
            .await;
        manager
            .add_message("sess_1", MessageRole::User, "how are you?")
            .await;
        let messages = manager.get_messages("sess_1").await;
        assert_eq!(messages.len(), 3);
    }

    #[tokio::test]
    async fn test_session_auto_trim() {
        let manager = SessionManager::new(5, 3, Duration::from_secs(1800));
        for i in 0..10 {
            manager
                .add_message("sess_1", MessageRole::User, format!("msg {i}"))
                .await;
        }
        let messages = manager.get_messages("sess_1").await;
        assert_eq!(messages.len(), 3);
        assert_eq!(messages[0].content, "msg 7");
    }

    #[tokio::test]
    async fn test_cleanup_idle_sessions() {
        let manager = SessionManager::new(50, 20, Duration::from_millis(100));
        manager
            .add_message("sess_1", MessageRole::User, "hello")
            .await;
        assert_eq!(manager.session_count().await, 1);

        // Wait for session to become idle
        tokio::time::sleep(Duration::from_millis(150)).await;
        let removed = manager.cleanup_idle().await;
        assert_eq!(removed, 1);
        assert_eq!(manager.session_count().await, 0);
    }

    #[tokio::test]
    async fn test_session_isolation() {
        let manager = SessionManager::new(50, 20, Duration::from_secs(1800));
        manager
            .add_message("sess_1", MessageRole::User, "hello")
            .await;
        manager
            .add_message("sess_2", MessageRole::User, "world")
            .await;

        let msgs1 = manager.get_messages("sess_1").await;
        let msgs2 = manager.get_messages("sess_2").await;
        assert_eq!(msgs1.len(), 1);
        assert_eq!(msgs2.len(), 1);
        assert_eq!(msgs1[0].content, "hello");
        assert_eq!(msgs2[0].content, "world");
    }
}
```

- [ ] **Step 2: Declare module in main.rs**

Edit `crates/golem-daemon/src/main.rs`:

```rust
use anyhow::Result;

pub mod chat;
mod config;
mod openai;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    tracing::info!("Golem daemon v{} starting", env!("CARGO_PKG_VERSION"));
    Ok(())
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cargo test -p golem-daemon --lib chat`
Expected: 6 tests PASS

- [ ] **Step 4: Run clippy + fmt**

Run: `cargo clippy -p golem-daemon --all-targets -- -D warnings && cargo fmt -p golem-daemon -- --check`
Expected: clean

- [ ] **Step 5: Commit**

```bash
git add crates/golem-daemon/src/chat.rs crates/golem-daemon/src/main.rs
git commit -m "feat(daemon): add chat session manager with auto-trim and idle cleanup"
```

---

## Task 7: WebSocket Server (golem-daemon)

**Files:**
- Create: `crates/golem-daemon/src/ws.rs`
- Modify: `crates/golem-daemon/src/main.rs`

- [ ] **Step 1: Write integration test for WebSocket handler**

Create `crates/golem-daemon/src/ws.rs`:

```rust
use axum::extract::ws::{Message, WebSocket};
use axum::extract::{State, WebSocketUpgrade};
use axum::response::IntoResponse;
use axum::routing::get;
use axum::Router;
use futures::{SinkExt, StreamExt};
use golem_core::llm::LlmProvider;
use golem_core::protocol::{ChatErrorResponse, ChatRequest, StreamDelta, StreamDone};
use golem_core::session::MessageRole;
use std::sync::Arc;

use crate::chat::SessionManager;

/// Shared application state accessible from WebSocket handlers.
pub struct AppState {
    pub session_manager: Arc<SessionManager>,
    pub llm: Arc<dyn LlmProvider>,
    pub chat_config: golem_core::llm::ChatConfig,
}

/// Build the axum router with WebSocket endpoint.
pub fn router(state: Arc<AppState>) -> Router {
    Router::new()
        .route("/ws", get(ws_handler))
        .with_state(state)
}

/// Handle WebSocket upgrade at /ws.
async fn ws_handler(
    ws: WebSocketUpgrade,
    State(state): State<Arc<AppState>>,
) -> impl IntoResponse {
    ws.on_upgrade(move |socket| handle_socket(socket, state))
}

/// Handle an individual WebSocket connection.
async fn handle_socket(mut socket: WebSocket, state: Arc<AppState>) {
    tracing::info!("New WebSocket connection");

    while let Some(msg) = socket.next().await {
        let msg = match msg {
            Ok(Message::Text(text)) => text,
            Ok(Message::Close(_)) => {
                tracing::info!("WebSocket closed by client");
                return;
            }
            Err(e) => {
                tracing::error!("WebSocket error: {e}");
                return;
            }
            _ => continue,
        };

        let request: ChatRequest = match serde_json::from_str(&msg) {
            Ok(req) => req,
            Err(e) => {
                tracing::warn!("Invalid message format: {e}");
                let err = ChatErrorResponse::new("", format!("Invalid message: {e}"));
                let _ = socket
                    .send(Message::Text(serde_json::to_string(&err).unwrap().into()))
                    .await;
                continue;
            }
        };

        let session_id = request.session_id.clone();
        let state = state.clone();
        let mut socket = socket;

        // Add user message to session
        state
            .session_manager
            .add_message(&session_id, MessageRole::User, &request.content)
            .await;

        // Get full conversation history
        let messages = state.session_manager.get_messages(&session_id).await;

        // Use chat config from AppState
        let config = state.chat_config.clone();

        // Call LLM with streaming
        match state.llm.chat_stream(&messages, &config).await {
            Ok(mut stream) => {
                let mut full_response = String::new();

                while let Some(delta) = stream.next().await {
                    match delta {
                        Ok(delta) => {
                            if let Some(content) = &delta.content {
                                full_response.push_str(content);
                                let stream_delta =
                                    StreamDelta::new(&session_id, content);
                                let _ = socket
                                    .send(Message::Text(
                                        serde_json::to_string(&stream_delta)
                                            .unwrap()
                                            .into(),
                                    ))
                                    .await;
                            }
                        }
                        Err(e) => {
                            tracing::error!("Stream error: {e}");
                            let err = ChatErrorResponse::new(&session_id, e.to_string());
                            let _ = socket
                                .send(Message::Text(
                                    serde_json::to_string(&err).unwrap().into(),
                                ))
                                .await;
                            return;
                        }
                    }
                }

                // Send done message
                let done = StreamDone::new(&session_id, &full_response);
                let _ = socket
                    .send(Message::Text(
                        serde_json::to_string(&done).unwrap().into(),
                    ))
                    .await;

                // Add assistant message to session
                state
                    .session_manager
                    .add_message(&session_id, MessageRole::Assistant, &full_response)
                    .await;
            }
            Err(e) => {
                tracing::error!("LLM call failed: {e}");
                let err = ChatErrorResponse::new(&session_id, e.to_string());
                let _ = socket
                    .send(Message::Text(
                        serde_json::to_string(&err).unwrap().into(),
                    ))
                    .await;
            }
        }
    }

    tracing::info!("WebSocket connection ended");
}

#[cfg(test)]
mod tests {
    use super::*;
    use golem_core::llm::{ChatConfig, ChatDelta};
    use golem_core::session::Message as CoreMessage;
    use futures::stream;

    /// Mock LLM provider for testing.
    struct MockLlmProvider {
        responses: Vec<String>,
    }

    impl MockLlmProvider {
        fn new(responses: Vec<String>) -> Self {
            Self { responses }
        }
    }

    #[async_trait::async_trait]
    impl LlmProvider for MockLlmProvider {
        fn name(&self) -> &str {
            "mock"
        }

        async fn chat(&self, _messages: &[CoreMessage], _config: &ChatConfig) -> anyhow::Result<String> {
            Ok(self.responses.first().cloned().unwrap_or_default())
        }

        async fn chat_stream(
            &self,
            _messages: &[CoreMessage],
            _config: &ChatConfig,
        ) -> anyhow::Result<std::pin::Pin<Box<dyn futures::Stream<Item = anyhow::Result<ChatDelta>> + Send>>>
        {
            let deltas: Vec<anyhow::Result<ChatDelta>> = self
                .responses
                .iter()
                .map(|r| {
                    Ok(ChatDelta {
                        content: Some(r.clone()),
                        finish_reason: None,
                    })
                })
                .chain(std::iter::once(Ok(ChatDelta {
                    content: None,
                    finish_reason: Some("stop".to_string()),
                })))
                .collect();
            Ok(Box::pin(stream::iter(deltas)))
        }
    }

    #[tokio::test]
    async fn test_mock_provider_name() {
        let provider = MockLlmProvider::new(vec!["hello".to_string()]);
        assert_eq!(provider.name(), "mock");
    }

    #[tokio::test]
    async fn test_mock_provider_stream() {
        let provider = MockLlmProvider::new(vec!["hello".to_string(), " world".to_string()]);
        let config = ChatConfig {
            model: "test".to_string(),
            system_prompt: None,
            temperature: None,
            max_tokens: None,
        };
        let mut stream = provider.chat_stream(&[], &config).await.unwrap();
        let mut result = String::new();
        while let Some(delta) = stream.next().await {
            let delta = delta.unwrap();
            if let Some(content) = delta.content {
                result.push_str(&content);
            }
        }
        assert_eq!(result, "hello world");
    }
}
```

- [ ] **Step 2: Declare module in main.rs**

Edit `crates/golem-daemon/src/main.rs`:

```rust
use anyhow::Result;

pub mod chat;
mod config;
mod openai;
pub mod ws;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    tracing::info!("Golem daemon v{} starting", env!("CARGO_PKG_VERSION"));
    Ok(())
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cargo test -p golem-daemon --lib ws`
Expected: 2 tests PASS

- [ ] **Step 4: Run clippy + fmt**

Run: `cargo clippy -p golem-daemon --all-targets -- -D warnings && cargo fmt -p golem-daemon -- --check`
Expected: clean

- [ ] **Step 5: Commit**

```bash
git add crates/golem-daemon/src/ws.rs crates/golem-daemon/src/main.rs
git commit -m "feat(daemon): add WebSocket server with chat request handling"
```

---

## Task 8: Daemon Main Entry (golem-daemon)

**Files:**
- Modify: `crates/golem-daemon/src/main.rs`

- [ ] **Step 1: Implement main entry point with config loading, server start, and graceful shutdown**

Replace `crates/golem-daemon/src/main.rs`:

```rust
use anyhow::Result;
use axum::Router;
use golem_core::llm::LlmProvider;
use std::sync::Arc;
use std::time::Duration;

pub mod chat;
mod config;
mod openai;
pub mod ws;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    tracing::info!("Golem daemon v{} starting", env!("CARGO_PKG_VERSION"));

    // Load config
    let app_config = match config::AppConfig::load() {
        Ok(config) => config,
        Err(e) => {
            tracing::warn!("Failed to load config: {e}, using defaults");
            config::AppConfig {
                server: config::ServerConfig {
                    port: 9921,
                    host: "0.0.0.0".to_string(),
                },
                llm: config::LlmConfig {
                    provider: "openai".to_string(),
                    api_key: std::env::var("OPENAI_API_KEY").unwrap_or_default(),
                    model: "gpt-4o".to_string(),
                    base_url: "https://api.openai.com/v1".to_string(),
                    system_prompt: None,
                    temperature: 0.7,
                    max_tokens: 4096,
                },
                session: config::SessionConfig::default(),
            }
        }
    };

    // Create LLM provider
    let llm: Arc<dyn LlmProvider> = Arc::new(openai::OpenAiProvider::new(
        app_config.llm.base_url.clone(),
        app_config.llm.api_key.clone(),
    ));

    // Create session manager
    let session_manager = Arc::new(chat::SessionManager::new(
        app_config.session.max_history,
        app_config.session.trim_to,
        Duration::from_secs(app_config.session.idle_timeout_minutes * 60),
    ));

    // Spawn idle session cleanup task
    chat::spawn_cleanup_task(session_manager.clone(), Duration::from_secs(5 * 60));

    // Build chat config from loaded LLM settings
    let chat_config = golem_core::llm::ChatConfig {
        model: app_config.llm.model.clone(),
        system_prompt: app_config.llm.system_prompt.clone(),
        temperature: Some(app_config.llm.temperature),
        max_tokens: Some(app_config.llm.max_tokens),
    };

    // Build shared state
    let state = Arc::new(ws::AppState {
        session_manager,
        llm,
        chat_config,
    });

    // Build router
    let app = ws::router(state);

    // Start server
    let addr = format!("{}:{}", app_config.server.host, app_config.server.port);
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    tracing::info!("Listening on {addr}");

    // Graceful shutdown
    let (tx, rx) = tokio::sync::watch::channel(());
    let server = axum::serve(listener, app).with_graceful_shutdown(shutdown_signal(rx));

    tokio::select! {
        result = server => {
            if let Err(e) = result {
                tracing::error!("Server error: {e}");
            }
        }
        _ = tokio::signal::ctrl_c() => {
            tracing::info!("Received Ctrl+C, shutting down...");
            drop(tx);
        }
    }

    tracing::info!("Golem daemon stopped");
    Ok(())
}

async fn shutdown_signal(mut rx: tokio::sync::watch::Receiver<()>) {
    // Wait for either ctrl-c or the watch channel to close
    tokio::select! {
        _ = tokio::signal::ctrl_c() => {}
        _ = rx.changed() => {}
    }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cargo check -p golem-daemon`
Expected: compiles without errors

- [ ] **Step 3: Run all daemon tests**

Run: `cargo test -p golem-daemon`
Expected: All tests PASS

- [ ] **Step 4: Run clippy + fmt**

Run: `cargo clippy -p golem-daemon --all-targets -- -D warnings && cargo fmt -p golem-daemon -- --check`
Expected: clean

- [ ] **Step 5: Commit**

```bash
git add crates/golem-daemon/src/main.rs
git commit -m "feat(daemon): wire up main entry with config, LLM provider, and graceful shutdown"
```

---

## Task 9: Feishu Gateway — Project Setup

**Files:**
- Create: `feishu-gateway/package.json`
- Create: `feishu-gateway/tsconfig.json`
- Create: `feishu-gateway/src/types.ts`

- [ ] **Step 1: Create package.json**

Create `feishu-gateway/package.json`:

```json
{
  "name": "feishu-gateway",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "start": "tsx src/index.ts",
    "build": "tsc",
    "typecheck": "tsc --noEmit",
    "test:unit": "vitest run"
  },
  "dependencies": {
    "@larksuiteoapi/node-sdk": "^1.40.0",
    "ws": "^8.18.0",
    "dotenv": "^16.4.0"
  },
  "devDependencies": {
    "typescript": "^5.7.0",
    "tsx": "^4.19.0",
    "vitest": "^3.0.0",
    "@types/ws": "^8.5.0",
    "@types/node": "^22.0.0"
  }
}
```

- [ ] **Step 2: Create tsconfig.json**

Create `feishu-gateway/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "outDir": "dist",
    "rootDir": "src",
    "declaration": true,
    "sourceMap": true
  },
  "include": ["src/**/*"]
}
```

- [ ] **Step 3: Create protocol types**

Create `feishu-gateway/src/types.ts`:

```typescript
// WebSocket protocol types matching golem-daemon JSON schema.

export interface ChatRequest {
  type: "chat_request";
  session_id: string;
  user_id: string;
  content: string;
  channel: "feishu";
}

export interface StreamDelta {
  type: "chat_delta";
  session_id: string;
  delta: string;
}

export interface StreamDone {
  type: "chat_done";
  session_id: string;
  full_response: string;
}

export interface ChatError {
  type: "chat_error";
  session_id: string;
  error: string;
}

export type DaemonMessage = StreamDelta | StreamDone | ChatError;

export function isStreamDelta(msg: DaemonMessage): msg is StreamDelta {
  return msg.type === "chat_delta";
}

export function isStreamDone(msg: DaemonMessage): msg is StreamDone {
  return msg.type === "chat_done";
}

export function isChatError(msg: DaemonMessage): msg is ChatError {
  return msg.type === "chat_error";
}
```

- [ ] **Step 4: Install dependencies and typecheck**

Run: `cd feishu-gateway && pnpm install && pnpm typecheck`
Expected: clean, no errors

- [ ] **Step 5: Commit**

```bash
git add feishu-gateway/
git commit -m "feat(gateway): initialize feishu-gateway TypeScript project with protocol types"
```

---

## Task 10: Feishu Gateway — Daemon Client

**Files:**
- Create: `feishu-gateway/src/daemon-client.ts`
- Create: `feishu-gateway/src/__tests__/daemon-client.test.ts`

- [ ] **Step 1: Write failing test for daemon client**

Create `feishu-gateway/src/__tests__/daemon-client.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { DaemonClient } from "../daemon-client.js";
import { WebSocket, WebSocketServer } from "ws";

describe("DaemonClient", () => {
  let wss: WebSocketServer;
  let port: number;

  beforeEach(async () => {
    wss = new WebSocketServer({ port: 0 });
    await new Promise<void>((resolve) => {
      wss.on("listening", () => {
        port = (wss.address() as any).port;
        resolve();
      });
    });
  });

  afterEach(() => {
    wss.close();
  });

  it("should connect and receive messages", async () => {
    const received: any[] = [];

    wss.on("connection", (ws) => {
      ws.on("message", (data) => {
        const msg = JSON.parse(data.toString());
        // Echo back as stream delta
        ws.send(
          JSON.stringify({
            type: "chat_delta",
            session_id: msg.session_id,
            delta: "echo: " + msg.content,
          })
        );
        ws.send(
          JSON.stringify({
            type: "chat_done",
            session_id: msg.session_id,
            full_response: "echo: " + msg.content,
          })
        );
      });
    });

    const client = new DaemonClient(`ws://localhost:${port}`);
    await client.connect();

    const response = await client.sendChat({
      type: "chat_request",
      session_id: "sess_1",
      user_id: "user_1",
      content: "hello",
      channel: "feishu",
    });

    expect(response.fullResponse).toBe("echo: hello");
    expect(response.deltas).toContain("echo: hello");

    client.disconnect();
  });

  it("should handle connection errors", async () => {
    const client = new DaemonClient("ws://localhost:99999");
    await expect(client.connect()).rejects.toThrow();
  });
});
```

- [ ] **Step 2: Implement daemon client**

Create `feishu-gateway/src/daemon-client.ts`:

```typescript
import WebSocket from "ws";
import type { ChatRequest, DaemonMessage, StreamDelta, StreamDone, ChatError } from "./types.js";
import { isStreamDelta, isStreamDone, isChatError } from "./types.js";

interface ChatResponse {
  deltas: string[];
  fullResponse: string;
  error?: string;
}

export class DaemonClient {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectBaseDelay = 1000;

  constructor(url: string) {
    this.url = url;
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url);

      this.ws.on("open", () => {
        this.reconnectAttempts = 0;
        resolve();
      });

      this.ws.on("error", (err) => {
        if (this.ws?.readyState === WebSocket.CONNECTING) {
          reject(err);
        }
      });

      this.ws.on("close", () => {
        this.scheduleReconnect();
      });
    });
  }

  private scheduleReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error("Max reconnect attempts reached");
      return;
    }

    const delay = this.reconnectBaseDelay * Math.pow(2, this.reconnectAttempts);
    this.reconnectAttempts++;

    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
    setTimeout(() => {
      this.connect().catch((err) => {
        console.error("Reconnect failed:", err.message);
      });
    }, delay);
  }

  sendChat(request: ChatRequest): Promise<ChatResponse> {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        reject(new Error("Not connected to daemon"));
        return;
      }

      const deltas: string[] = [];
      let fullResponse = "";

      const handler = (data: WebSocket.Data) => {
        const msg: DaemonMessage = JSON.parse(data.toString());

        if (msg.session_id !== request.session_id) return;

        if (isStreamDelta(msg)) {
          deltas.push(msg.delta);
        } else if (isStreamDone(msg)) {
          fullResponse = msg.full_response;
          this.ws?.removeListener("message", handler);
          resolve({ deltas, fullResponse });
        } else if (isChatError(msg)) {
          this.ws?.removeListener("message", handler);
          resolve({ deltas, fullResponse, error: msg.error });
        }
      };

      this.ws.on("message", handler);
      this.ws.send(JSON.stringify(request));

      // Timeout after 60 seconds
      setTimeout(() => {
        this.ws?.removeListener("message", handler);
        reject(new Error("Chat request timed out"));
      }, 60_000);
    });
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd feishu-gateway && pnpm test:unit`
Expected: 2 tests PASS

- [ ] **Step 4: Typecheck**

Run: `cd feishu-gateway && pnpm typecheck`
Expected: clean

- [ ] **Step 5: Commit**

```bash
git add feishu-gateway/src/daemon-client.ts feishu-gateway/src/__tests__/daemon-client.test.ts
git commit -m "feat(gateway): add daemon WebSocket client with reconnect logic"
```

---

## Task 11: Feishu Gateway — Feishu Connection

**Files:**
- Create: `feishu-gateway/src/feishu.ts`
- Create: `feishu-gateway/.env.example`

- [ ] **Step 1: Create .env.example**

Create `feishu-gateway/.env.example`:

```env
# Feishu App credentials
FEISHU_APP_ID=cli_xxx
FEISHU_APP_SECRET=xxx
FEISHU_VERIFICATION_TOKEN=xxx

# Daemon WebSocket URL
DAEMON_WS_URL=ws://localhost:9921/ws
```

- [ ] **Step 2: Implement Feishu connection module**

Create `feishu-gateway/src/feishu.ts`:

```typescript
import * as lark from "@larksuiteoapi/node-sdk";

interface FeishuConfig {
  appId: string;
  appSecret: string;
  verificationToken: string;
}

interface FeishuMessageEvent {
  message_id: string;
  chat_id: string;
  chat_type: "p2p" | "group";
  content: string;
  sender: {
    sender_id: {
      open_id: string;
    };
  };
  mentions?: Array<{
    key: string;
    id: { open_id: string };
  }>;
}

export type MessageHandler = (event: FeishuMessageEvent) => Promise<void>;

export class FeishuConnection {
  private client: lark.Client;
  private wsClient: lark.WSClient | null = null;
  private config: FeishuConfig;
  private handler: MessageHandler | null = null;
  private recentMessageIds: Map<string, number> = new Map();
  private dedupTtl = 5 * 60 * 1000; // 5 minutes

  constructor(config: FeishuConfig) {
    this.config = config;
    this.client = new lark.Client({
      appId: config.appId,
      appSecret: config.appSecret,
    });
  }

  onMessage(handler: MessageHandler) {
    this.handler = handler;
  }

  async connect() {
    this.wsClient = new lark.WSClient({
      appId: this.config.appId,
      appSecret: this.config.appSecret,
    });

    this.wsClient.start({
      eventDispatcher: new lark.EventDispatcher({
        verificationToken: this.config.verificationToken,
      }).register({
        "im.message.receive_v1": async (data: any) => {
          const event = data.event as FeishuMessageEvent;

          // Dedup check
          if (this.isDuplicate(event.message_id)) {
            return;
          }

          // Group chat: only respond when mentioned
          if (event.chat_type === "group") {
            const isMentioned =
              event.mentions?.some(
                (m) => m.id.open_id === this.client.appId
              ) ?? false;
            if (!isMentioned) return;
          }

          if (this.handler) {
            await this.handler(event);
          }
        },
      }),
    });

    console.log("Feishu WebSocket connected");
  }

  private isDuplicate(messageId: string): boolean {
    this.cleanupExpired();
    if (this.recentMessageIds.has(messageId)) {
      return true;
    }
    this.recentMessageIds.set(messageId, Date.now());
    return false;
  }

  private cleanupExpired() {
    const now = Date.now();
    for (const [id, timestamp] of this.recentMessageIds) {
      if (now - timestamp > this.dedupTtl) {
        this.recentMessageIds.delete(id);
      }
    }
  }

  async sendMessage(chatId: string, content: string) {
    await this.client.im.message.create({
      params: { receive_id_type: "chat_id" },
      data: {
        receive_id: chatId,
        msg_type: "text",
        content: JSON.stringify({ text: content }),
      },
    });
  }

  async replyMessage(messageId: string, content: string) {
    await this.client.im.message.reply({
      path: { message_id: messageId },
      data: {
        msg_type: "text",
        content: JSON.stringify({ text: content }),
      },
    });
  }

  getClient(): lark.Client {
    return this.client;
  }
}
```

- [ ] **Step 3: Typecheck**

Run: `cd feishu-gateway && pnpm typecheck`
Expected: clean

- [ ] **Step 4: Commit**

```bash
git add feishu-gateway/src/feishu.ts feishu-gateway/.env.example
git commit -m "feat(gateway): add Feishu WSClient connection with message dedup"
```

---

## Task 12: Feishu Gateway — Streaming + Handler + Entry

**Files:**
- Create: `feishu-gateway/src/streaming.ts`
- Create: `feishu-gateway/src/handler.ts`
- Create: `feishu-gateway/src/index.ts`
- Create: `feishu-gateway/.gitignore`

- [ ] **Step 1: Create .gitignore**

Create `feishu-gateway/.gitignore`:

```
node_modules/
dist/
.env
```

- [ ] **Step 2: Implement Card Kit streaming**

Create `feishu-gateway/src/streaming.ts`:

```typescript
import type { Client } from "@larksuiteoapi/node-sdk";

interface StreamingCard {
  cardId: string;
  content: string;
  lastUpdateTime: number;
  minInterval: number;
  minChars: number;
}

export class StreamingManager {
  private cards: Map<string, StreamingCard> = new Map();
  private client: Client;
  private fallbackHandler: (sessionId: string, content: string) => Promise<void>;

  constructor(
    client: Client,
    fallbackHandler: (sessionId: string, content: string) => Promise<void>
  ) {
    this.client = client;
    this.fallbackHandler = fallbackHandler;
  }

  async onDelta(sessionId: string, chatId: string, messageId: string, delta: string) {
    let card = this.cards.get(sessionId);

    if (!card) {
      // Create new streaming card
      try {
        card = await this.createCard(chatId, messageId);
        this.cards.set(sessionId, card);
      } catch (e) {
        console.error("Failed to create streaming card:", e);
        // Will fall back to regular reply on done
        return;
      }
    }

    card.content += delta;

    // Throttle updates
    const now = Date.now();
    if (
      now - card.lastUpdateTime >= card.minInterval &&
      card.content.length >= card.minChars
    ) {
      await this.updateCard(card);
      card.lastUpdateTime = now;
    }
  }

  async onDone(sessionId: string, chatId: string, messageId: string, fullResponse: string) {
    const card = this.cards.get(sessionId);

    if (card) {
      // Final update with complete content
      card.content = fullResponse;
      await this.updateCard(card);
      await this.closeCard(card);
      this.cards.delete(sessionId);
    } else {
      // Fallback to regular reply
      await this.fallbackHandler(sessionId, fullResponse);
    }
  }

  private async createCard(chatId: string, messageId: string): Promise<StreamingCard> {
    // Card Kit streaming create
    const response = await (this.client as any).cardkit.v1.card.create({
      data: {
        type: "card_kit",
        data: {
          card_link: {
            url: `https://open.feishu.cn/open-apis/cardkit/v1/cards/{card_id}/elements/content/content`,
          },
          elements: [
            {
              tag: "markdown",
              content: "",
              element_id: "content",
            },
          ],
        },
      },
    });

    return {
      cardId: response.card_id || "",
      content: "",
      lastUpdateTime: Date.now(),
      minInterval: 160,
      minChars: 18,
    };
  }

  private async updateCard(card: StreamingCard) {
    try {
      await (this.client as any).cardkit.v1.cardElement.update({
        path: { card_id: card.cardId, element_id: "content" },
        data: {
          content: card.content,
        },
      });
    } catch (e) {
      console.error("Failed to update card:", e);
    }
  }

  private async closeCard(card: StreamingCard) {
    try {
      await (this.client as any).cardkit.v1.card.patch({
        path: { card_id: card.cardId },
        data: {
          type: "card_kit",
          data: {
            elements: [
              {
                tag: "markdown",
                content: card.content,
                element_id: "content",
              },
            ],
          },
        },
      });
    } catch (e) {
      console.error("Failed to close card:", e);
    }
  }
}
```

- [ ] **Step 3: Implement message handler**

Create `feishu-gateway/src/handler.ts`:

```typescript
import { FeishuConnection } from "./feishu.js";
import { DaemonClient } from "./daemon-client.js";
import { StreamingManager } from "./streaming.js";
import type { ChatRequest } from "./types.js";

export class MessageHandler {
  private feishu: FeishuConnection;
  private daemon: DaemonClient;
  private streaming: StreamingManager;

  constructor(feishu: FeishuConnection, daemon: DaemonClient) {
    this.feishu = feishu;
    this.daemon = daemon;
    this.streaming = new StreamingManager(
      feishu.getClient(),
      async (sessionId, content) => {
        // Fallback: send as regular message (requires storing chatId mapping)
        console.log(`Fallback reply for ${sessionId}: ${content}`);
      }
    );
  }

  async handleMessage(event: {
    message_id: string;
    chat_id: string;
    chat_type: string;
    content: string;
    sender: { sender_id: { open_id: string } };
  }) {
    const { message_id, chat_id, chat_type, sender } = event;
    const userId = sender.sender_id.open_id;

    // Parse message content
    let content: string;
    try {
      const parsed = JSON.parse(event.content);
      content = parsed.text || "";
    } catch {
      content = event.content;
    }

    // Strip @bot mentions from group messages
    if (chat_type === "group") {
      content = content.replace(/@_user_\d+/g, "").trim();
    }

    if (!content) return;

    // Build session ID
    const sessionId =
      chat_type === "p2p"
        ? `feishu:${userId}`
        : `feishu:group:${chat_id}:${userId}`;

    // Build daemon request
    const request: ChatRequest = {
      type: "chat_request",
      session_id: sessionId,
      user_id: userId,
      content,
      channel: "feishu",
    };

    try {
      const response = await this.daemon.sendChat(request);

      if (response.error) {
        await this.feishu.replyMessage(message_id, `Error: ${response.error}`);
        return;
      }

      // Streaming was handled via deltas
      if (response.deltas.length > 0) {
        await this.streaming.onDone(
          sessionId,
          chat_id,
          message_id,
          response.fullResponse
        );
      } else {
        await this.feishu.replyMessage(message_id, response.fullResponse);
      }
    } catch (e) {
      console.error("Failed to handle message:", e);
      await this.feishu.replyMessage(message_id, "Sorry, something went wrong.");
    }
  }
}
```

- [ ] **Step 4: Implement main entry point**

Create `feishu-gateway/src/index.ts`:

```typescript
import "dotenv/config";
import { FeishuConnection } from "./feishu.js";
import { DaemonClient } from "./daemon-client.js";
import { MessageHandler } from "./handler.js";

async function main() {
  const feishuAppId = process.env.FEISHU_APP_ID;
  const feishuAppSecret = process.env.FEISHU_APP_SECRET;
  const feishuVerificationToken = process.env.FEISHU_VERIFICATION_TOKEN;
  const daemonWsUrl = process.env.DAEMON_WS_URL || "ws://localhost:9921/ws";

  if (!feishuAppId || !feishuAppSecret || !feishuVerificationToken) {
    console.error(
      "Missing required env vars: FEISHU_APP_ID, FEISHU_APP_SECRET, FEISHU_VERIFICATION_TOKEN"
    );
    process.exit(1);
  }

  // Connect to daemon
  const daemon = new DaemonClient(daemonWsUrl);
  try {
    await daemon.connect();
    console.log(`Connected to daemon at ${daemonWsUrl}`);
  } catch (e) {
    console.error("Failed to connect to daemon:", e);
    process.exit(1);
  }

  // Connect to Feishu
  const feishu = new FeishuConnection({
    appId: feishuAppId,
    appSecret: feishuAppSecret,
    verificationToken: feishuVerificationToken,
  });

  const handler = new MessageHandler(feishu, daemon);
  feishu.onMessage(async (event) => {
    await handler.handleMessage(event);
  });

  await feishu.connect();
  console.log("Feishu gateway started");

  // Graceful shutdown
  const shutdown = () => {
    console.log("Shutting down...");
    daemon.disconnect();
    process.exit(0);
  };

  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}

main().catch((e) => {
  console.error("Fatal error:", e);
  process.exit(1);
});
```

- [ ] **Step 5: Typecheck**

Run: `cd feishu-gateway && pnpm typecheck`
Expected: clean

- [ ] **Step 6: Commit**

```bash
git add feishu-gateway/src/streaming.ts feishu-gateway/src/handler.ts feishu-gateway/src/index.ts feishu-gateway/.gitignore
git commit -m "feat(gateway): add streaming manager, message handler, and entry point"
```

---

## Task 13: Integration — Full Build Verification

- [ ] **Step 1: Run full Rust test suite**

Run: `cargo test --workspace`
Expected: All tests PASS

- [ ] **Step 2: Run clippy on entire workspace**

Run: `cargo clippy --workspace --all-targets -- -D warnings`
Expected: clean

- [ ] **Step 3: Run fmt check**

Run: `cargo fmt --all -- --check`
Expected: clean

- [ ] **Step 4: Run feishu-gateway typecheck**

Run: `cd feishu-gateway && pnpm typecheck`
Expected: clean

- [ ] **Step 5: Run local quality gate**

Run: `sh test.sh`
Expected: All checks passed

- [ ] **Step 6: Commit any fixes**

```bash
git add -A
git commit -m "chore: fix formatting and clippy issues from integration check"
```

---

## Execution Order Summary

| Task | Component | Dependencies |
|------|-----------|-------------|
| 1 | golem-core: LLM trait + types | None |
| 2 | golem-core: Protocol types | None (parallel with 1) |
| 3 | golem-core: Session update | Task 1 (uses imports) |
| 4 | golem-daemon: Config module | None |
| 5 | golem-daemon: OpenAI provider | Task 1 |
| 6 | golem-daemon: Session manager | Task 3 |
| 7 | golem-daemon: WebSocket server | Tasks 2, 5, 6 |
| 8 | golem-daemon: Main entry | Tasks 4, 7 |
| 9 | feishu-gateway: Setup | None |
| 10 | feishu-gateway: Daemon client | Task 9 |
| 11 | feishu-gateway: Feishu connection | Task 9 |
| 12 | feishu-gateway: Streaming + handler | Tasks 10, 11 |
| 13 | Integration verification | All tasks |
