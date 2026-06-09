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

/// Inner payload for StreamDelta (without type field, used by DaemonMessage).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StreamDeltaInner {
    pub session_id: String,
    pub delta: String,
}

/// Inner payload for StreamDone (without type field, used by DaemonMessage).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StreamDoneInner {
    pub session_id: String,
    pub full_response: String,
}

/// Inner payload for ChatErrorResponse (without type field, used by DaemonMessage).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatErrorInner {
    pub session_id: String,
    pub error: String,
}

/// Discriminated union for all daemon -> gateway messages.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum DaemonMessage {
    #[serde(rename = "chat_delta")]
    StreamDelta(StreamDeltaInner),
    #[serde(rename = "chat_done")]
    StreamDone(StreamDoneInner),
    #[serde(rename = "chat_error")]
    ChatError(ChatErrorInner),
}

impl DaemonMessage {
    pub fn stream_delta(session_id: impl Into<String>, delta: impl Into<String>) -> Self {
        Self::StreamDelta(StreamDeltaInner {
            session_id: session_id.into(),
            delta: delta.into(),
        })
    }

    pub fn stream_done(session_id: impl Into<String>, full_response: impl Into<String>) -> Self {
        Self::StreamDone(StreamDoneInner {
            session_id: session_id.into(),
            full_response: full_response.into(),
        })
    }

    pub fn chat_error(session_id: impl Into<String>, error: impl Into<String>) -> Self {
        Self::ChatError(ChatErrorInner {
            session_id: session_id.into(),
            error: error.into(),
        })
    }

    pub fn session_id(&self) -> &str {
        match self {
            DaemonMessage::StreamDelta(d) => &d.session_id,
            DaemonMessage::StreamDone(d) => &d.session_id,
            DaemonMessage::ChatError(e) => &e.session_id,
        }
    }
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
        let msg = DaemonMessage::stream_delta("sess_1", "hi");
        let json = serde_json::to_string(&msg).unwrap();
        assert!(json.contains("\"type\":\"chat_delta\""));
        assert!(json.contains("\"delta\":\"hi\""));
        let deserialized: DaemonMessage = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.session_id(), "sess_1");
        match deserialized {
            DaemonMessage::StreamDelta(d) => assert_eq!(d.delta, "hi"),
            _ => panic!("wrong variant"),
        }
    }

    #[test]
    fn test_daemon_message_stream_done() {
        let msg = DaemonMessage::stream_done("sess_2", "full response");
        let json = serde_json::to_string(&msg).unwrap();
        assert!(json.contains("\"type\":\"chat_done\""));
        let deserialized: DaemonMessage = serde_json::from_str(&json).unwrap();
        match deserialized {
            DaemonMessage::StreamDone(d) => assert_eq!(d.full_response, "full response"),
            _ => panic!("wrong variant"),
        }
    }

    #[test]
    fn test_daemon_message_chat_error() {
        let msg = DaemonMessage::chat_error("sess_3", "timeout");
        let json = serde_json::to_string(&msg).unwrap();
        assert!(json.contains("\"type\":\"chat_error\""));
        let deserialized: DaemonMessage = serde_json::from_str(&json).unwrap();
        match deserialized {
            DaemonMessage::ChatError(e) => assert_eq!(e.error, "timeout"),
            _ => panic!("wrong variant"),
        }
    }
}
