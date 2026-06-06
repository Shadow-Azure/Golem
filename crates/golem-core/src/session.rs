use serde::{Deserialize, Serialize};
use std::time::{Duration, Instant};

/// Represents a conversation session.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    pub id: String,
    pub task_id: Option<String>,
    pub messages: Vec<Message>,
    #[serde(skip, default = "Instant::now")]
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
