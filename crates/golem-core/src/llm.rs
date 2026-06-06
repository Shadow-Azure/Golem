use async_trait::async_trait;
use futures::Stream;
use serde::{Deserialize, Serialize};
use std::pin::Pin;

use crate::session::Message;

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

/// Convert golem-core Message to OpenAI-format message dict.
pub fn messages_to_openai(
    messages: &[Message],
    system_prompt: Option<&str>,
) -> Vec<serde_json::Value> {
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
