use axum::extract::ws::{Message, WebSocket};
use axum::extract::{State, WebSocketUpgrade};
use axum::response::IntoResponse;
use axum::routing::get;
use axum::Router;
use futures::StreamExt;
use golem_core::llm::{ChatConfig, ChatDelta, LlmProvider};
use golem_core::protocol::{ChatErrorResponse, ChatRequest, StreamDelta, StreamDone};
use golem_core::session::MessageRole;
use std::sync::Arc;

use crate::chat::SessionManager;

/// Shared application state for WebSocket handlers.
pub struct AppState {
    pub session_manager: Arc<SessionManager>,
    pub llm: Arc<dyn LlmProvider>,
    pub chat_config: ChatConfig,
}

/// Builds the axum Router with the WebSocket endpoint.
pub fn router(state: Arc<AppState>) -> Router {
    Router::new()
        .route("/ws", get(ws_handler))
        .with_state(state)
}

/// Handles the HTTP -> WebSocket upgrade request.
async fn ws_handler(ws: WebSocketUpgrade, State(state): State<Arc<AppState>>) -> impl IntoResponse {
    ws.on_upgrade(move |socket| handle_socket(socket, state))
}

/// Per-connection WebSocket handler.
///
/// Reads text messages, parses them as ChatRequest, processes them through
/// the session manager and LLM provider with streaming responses.
async fn handle_socket(mut socket: WebSocket, state: Arc<AppState>) {
    while let Some(msg) = socket.recv().await {
        match msg {
            Ok(Message::Text(text)) => {
                let chat_req: ChatRequest = match serde_json::from_str(&text) {
                    Ok(req) => req,
                    Err(e) => {
                        let error = ChatErrorResponse::new("", format!("invalid request: {e}"));
                        let _ = socket
                            .send(Message::Text(serde_json::to_string(&error).unwrap()))
                            .await;
                        continue;
                    }
                };

                let session_id = chat_req.session_id.clone();

                // Add user message to session
                state
                    .session_manager
                    .add_message(&session_id, MessageRole::User, &chat_req.content)
                    .await;

                // Get conversation history
                let messages = state.session_manager.get_messages(&session_id).await;

                // Use the shared chat config
                let config = state.chat_config.clone();

                // Call LLM with streaming
                match state.llm.chat_stream(&messages, &config).await {
                    Ok(mut stream) => {
                        let mut full_response = String::new();

                        while let Some(item) = stream.next().await {
                            match item {
                                Ok(ChatDelta {
                                    content: Some(delta),
                                    ..
                                }) => {
                                    full_response.push_str(&delta);
                                    let msg = StreamDelta::new(&session_id, &delta);
                                    let _ = socket
                                        .send(Message::Text(serde_json::to_string(&msg).unwrap()))
                                        .await;
                                }
                                Ok(ChatDelta {
                                    finish_reason: Some(_),
                                    ..
                                }) => {
                                    // Stream complete
                                }
                                Ok(ChatDelta {
                                    content: None,
                                    finish_reason: None,
                                    ..
                                }) => {
                                    // Empty delta, ignore
                                }
                                Err(e) => {
                                    let error = ChatErrorResponse::new(
                                        &session_id,
                                        format!("stream error: {e}"),
                                    );
                                    let _ = socket
                                        .send(Message::Text(serde_json::to_string(&error).unwrap()))
                                        .await;
                                    break;
                                }
                            }
                        }

                        // Send StreamDone
                        let done = StreamDone::new(&session_id, &full_response);
                        let _ = socket
                            .send(Message::Text(serde_json::to_string(&done).unwrap()))
                            .await;

                        // Add assistant response to session
                        state
                            .session_manager
                            .add_message(&session_id, MessageRole::Assistant, &full_response)
                            .await;
                    }
                    Err(e) => {
                        let error = ChatErrorResponse::new(&session_id, format!("LLM error: {e}"));
                        let _ = socket
                            .send(Message::Text(serde_json::to_string(&error).unwrap()))
                            .await;
                    }
                }
            }
            Ok(Message::Close(_)) => {
                break;
            }
            Ok(_) => {
                // Ignore binary, ping, pong messages
            }
            Err(_) => {
                break;
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use async_trait::async_trait;
    use futures::{sink::SinkExt, stream};
    use golem_core::session::Message;
    use std::pin::Pin;
    use std::time::Duration;
    use tokio::net::TcpListener;
    use tokio_tungstenite::tungstenite::Message as TungsteniteMessage;

    /// Mock LLM provider for testing.
    struct MockLlmProvider {
        name: String,
        deltas: Vec<String>,
    }

    impl MockLlmProvider {
        fn new(name: &str, deltas: Vec<String>) -> Self {
            Self {
                name: name.to_string(),
                deltas,
            }
        }
    }

    #[async_trait]
    impl LlmProvider for MockLlmProvider {
        fn name(&self) -> &str {
            &self.name
        }

        async fn chat(
            &self,
            _messages: &[Message],
            _config: &ChatConfig,
        ) -> anyhow::Result<String> {
            Ok(self.deltas.join(""))
        }

        async fn chat_stream(
            &self,
            _messages: &[Message],
            _config: &ChatConfig,
        ) -> anyhow::Result<Pin<Box<dyn futures::Stream<Item = anyhow::Result<ChatDelta>> + Send>>>
        {
            let deltas: Vec<anyhow::Result<ChatDelta>> = self
                .deltas
                .iter()
                .map(|d| {
                    Ok(ChatDelta {
                        content: Some(d.clone()),
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

    #[test]
    fn test_mock_provider_name() {
        let provider = MockLlmProvider::new("mock-test", vec![]);
        assert_eq!(provider.name(), "mock-test");
    }

    #[tokio::test]
    async fn test_mock_provider_stream() {
        let provider =
            MockLlmProvider::new("mock", vec!["hello".to_string(), " world".to_string()]);
        let config = ChatConfig {
            model: "test-model".to_string(),
            system_prompt: None,
            temperature: None,
            max_tokens: None,
        };

        let result = provider.chat_stream(&[], &config).await.unwrap();
        let collected: Vec<anyhow::Result<ChatDelta>> = result.collect().await;

        let mut full_text = String::new();
        for d in collected.iter().flatten() {
            if let Some(ref content) = d.content {
                full_text.push_str(content);
            }
        }

        assert_eq!(full_text, "hello world");
    }

    #[tokio::test]
    async fn test_router_builds_successfully() {
        let session_manager = Arc::new(SessionManager::new(50, 20, Duration::from_secs(300)));
        let llm: Arc<dyn LlmProvider> =
            Arc::new(MockLlmProvider::new("mock", vec!["hi".to_string()]));
        let chat_config = ChatConfig {
            model: "test".to_string(),
            system_prompt: None,
            temperature: None,
            max_tokens: None,
        };
        let state = Arc::new(AppState {
            session_manager,
            llm,
            chat_config,
        });

        let _app = router(state);
    }

    #[tokio::test]
    async fn test_ws_chat_handler() {
        let session_manager = Arc::new(SessionManager::new(50, 20, Duration::from_secs(300)));
        let llm: Arc<dyn LlmProvider> = Arc::new(MockLlmProvider::new(
            "mock",
            vec!["hello".to_string(), " world".to_string()],
        ));
        let chat_config = ChatConfig {
            model: "test".to_string(),
            system_prompt: None,
            temperature: None,
            max_tokens: None,
        };
        let state = Arc::new(AppState {
            session_manager,
            llm,
            chat_config,
        });

        let app = router(state);

        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();

        tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });

        // Connect via WebSocket
        let url = format!("ws://{}:{}/ws", addr.ip(), addr.port());
        let (mut ws_stream, _) = tokio_tungstenite::connect_async(&url)
            .await
            .expect("failed to connect");

        // Send a chat request
        let chat_req = ChatRequest::new("test-session", "user-1", "hello", "test");
        let req_json = serde_json::to_string(&chat_req).unwrap();
        ws_stream
            .send(TungsteniteMessage::Text(req_json))
            .await
            .expect("failed to send message");

        // Collect responses
        let mut deltas = String::new();
        let mut got_done = false;

        // We expect: StreamDelta("hello"), StreamDelta(" world"), StreamDone
        for _ in 0..10 {
            let msg = tokio::time::timeout(Duration::from_secs(5), ws_stream.next())
                .await
                .expect("timeout waiting for message");

            match msg {
                Some(Ok(TungsteniteMessage::Text(text))) => {
                    let text = text.to_string();
                    // Try to parse as DaemonMessage (tagged enum)
                    if let Ok(daemon_msg) =
                        serde_json::from_str::<golem_core::protocol::DaemonMessage>(&text)
                    {
                        match daemon_msg {
                            golem_core::protocol::DaemonMessage::StreamDelta(d) => {
                                deltas.push_str(&d.delta);
                            }
                            golem_core::protocol::DaemonMessage::StreamDone(d) => {
                                assert_eq!(d.full_response, "hello world");
                                got_done = true;
                                break;
                            }
                            golem_core::protocol::DaemonMessage::ChatError(e) => {
                                panic!("unexpected error: {}", e.error);
                            }
                        }
                    }
                }
                Some(Ok(TungsteniteMessage::Close(_))) => {
                    break;
                }
                Some(Err(e)) => {
                    panic!("websocket error: {e}");
                }
                None => {
                    break;
                }
                _ => {}
            }
        }

        assert!(got_done, "should have received StreamDone");
        assert_eq!(deltas, "hello world");
    }
}
