use async_trait::async_trait;
use futures::Stream;
use golem_core::llm::{ChatConfig, ChatDelta, LlmProvider};
use golem_core::session::Message;
use reqwest::Client;
use serde::Deserialize;
use std::pin::Pin;
use std::time::Duration;

#[allow(dead_code)]
pub struct OpenAiProvider {
    client: Client,
    base_url: String,
    api_key: String,
}

#[allow(dead_code)]
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

// Internal deserialization types for the OpenAI chat completion response.
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
        let openai_messages =
            golem_core::llm::messages_to_openai(messages, config.system_prompt.as_deref());
        let mut body = serde_json::json!({
            "model": config.model,
            "messages": openai_messages,
        });
        if let Some(temp) = config.temperature {
            body["temperature"] = serde_json::json!(temp);
        }
        if let Some(max_tokens) = config.max_tokens {
            body["max_tokens"] = serde_json::json!(max_tokens);
        }

        let url = format!("{}/chat/completions", self.base_url);
        let response = self
            .client
            .post(&url)
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&body)
            .send()
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let text = response.text().await.unwrap_or_default();
            anyhow::bail!("OpenAI API error {}: {}", status, text);
        }

        let parsed: ChatCompletionResponse = response.json().await?;
        let content = parsed
            .choices
            .into_iter()
            .next()
            .map(|c| c.message.content)
            .unwrap_or_default();
        Ok(content)
    }

    async fn chat_stream(
        &self,
        messages: &[Message],
        config: &ChatConfig,
    ) -> anyhow::Result<Pin<Box<dyn Stream<Item = anyhow::Result<ChatDelta>> + Send>>> {
        let openai_messages =
            golem_core::llm::messages_to_openai(messages, config.system_prompt.as_deref());
        let mut body = serde_json::json!({
            "model": config.model,
            "messages": openai_messages,
            "stream": true,
        });
        if let Some(temp) = config.temperature {
            body["temperature"] = serde_json::json!(temp);
        }
        if let Some(max_tokens) = config.max_tokens {
            body["max_tokens"] = serde_json::json!(max_tokens);
        }

        let url = format!("{}/chat/completions", self.base_url);
        let response = self
            .client
            .post(&url)
            .header("Authorization", format!("Bearer {}", self.api_key))
            .json(&body)
            .send()
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let text = response.text().await.unwrap_or_default();
            anyhow::bail!("OpenAI API error {}: {}", status, text);
        }

        // Convert the response body byte stream into a line-based stream,
        // then parse SSE data lines into ChatDelta items.
        use futures::StreamExt;
        use tokio_util::io::StreamReader;

        let line_reader = StreamReader::new(
            response
                .bytes_stream()
                .map(|r| r.map_err(std::io::Error::other)),
        );

        use futures::stream::unfold;
        use tokio::io::AsyncBufReadExt;

        let buf_reader = tokio::io::BufReader::new(line_reader);
        let lines = unfold(buf_reader, |mut reader| async move {
            let mut line = String::new();
            match reader.read_line(&mut line).await {
                Ok(0) => None, // EOF
                Ok(_) => Some((line.trim_end().to_string(), reader)),
                Err(_) => None,
            }
        });

        let deltas = lines.filter_map(|line| async move {
            // Skip empty lines
            let trimmed = line.trim();
            if trimmed.is_empty() {
                return None;
            }

            // Extract SSE data payload
            let data = match trimmed.strip_prefix("data: ") {
                Some(d) => d,
                None => return None,
            };

            // Check for stream end
            if data == "[DONE]" {
                return Some(Ok(ChatDelta {
                    content: None,
                    finish_reason: Some("stop".to_string()),
                }));
            }

            // Parse the JSON chunk
            match serde_json::from_str::<StreamResponse>(data) {
                Ok(parsed) => parsed.choices.into_iter().next().map(|choice| {
                    Ok(ChatDelta {
                        content: choice.delta.content,
                        finish_reason: choice.finish_reason,
                    })
                }),
                Err(e) => Some(Err(anyhow::anyhow!("failed to parse stream chunk: {}", e))),
            }
        });

        Ok(Box::pin(deltas))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_openai_provider_name() {
        let provider = OpenAiProvider::new(
            "https://api.openai.com/v1".to_string(),
            "sk-test-key".to_string(),
        );
        assert_eq!(provider.name(), "openai");
    }
}
