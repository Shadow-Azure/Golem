use serde::{Deserialize, Serialize};

/// Represents a conversation session.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Session {
    pub id: String,
    pub task_id: String,
    pub messages: Vec<Message>,
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
    pub fn new(id: impl Into<String>, task_id: impl Into<String>) -> Self {
        Self {
            id: id.into(),
            task_id: task_id.into(),
            messages: Vec::new(),
        }
    }

    pub fn add_message(&mut self, role: MessageRole, content: impl Into<String>) {
        self.messages.push(Message {
            role,
            content: content.into(),
        });
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_session_creation() {
        let session = Session::new("s1", "t1");
        assert!(session.messages.is_empty());
    }

    #[test]
    fn test_add_message() {
        let mut session = Session::new("s1", "t1");
        session.add_message(MessageRole::User, "hello");
        assert_eq!(session.messages.len(), 1);
    }
}
