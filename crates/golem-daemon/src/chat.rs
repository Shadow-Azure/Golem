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
    use std::time::Duration;

    #[tokio::test]
    async fn test_get_or_create_session() {
        let mgr = SessionManager::new(100, 50, Duration::from_secs(300));
        let session = mgr.get_or_create("s1").await;
        assert_eq!(session.id, "s1");
        assert!(session.messages.is_empty());
    }

    #[tokio::test]
    async fn test_add_message_creates_session() {
        let mgr = SessionManager::new(100, 50, Duration::from_secs(300));
        mgr.add_message("s1", MessageRole::User, "hello").await;
        let msgs = mgr.get_messages("s1").await;
        assert_eq!(msgs.len(), 1);
        assert_eq!(msgs[0].content, "hello");
    }

    #[tokio::test]
    async fn test_multi_turn_conversation() {
        let mgr = SessionManager::new(100, 50, Duration::from_secs(300));
        mgr.add_message("s1", MessageRole::User, "hello").await;
        mgr.add_message("s1", MessageRole::Assistant, "hi there")
            .await;
        mgr.add_message("s1", MessageRole::User, "how are you?")
            .await;

        let msgs = mgr.get_messages("s1").await;
        assert_eq!(msgs.len(), 3);
        assert_eq!(msgs[0].content, "hello");
        assert_eq!(msgs[1].content, "hi there");
        assert_eq!(msgs[2].content, "how are you?");
    }

    #[tokio::test]
    async fn test_session_auto_trim() {
        let mgr = SessionManager::new(5, 3, Duration::from_secs(300));
        for i in 0..10 {
            mgr.add_message("s1", MessageRole::User, &format!("msg {i}"))
                .await;
        }
        let msgs = mgr.get_messages("s1").await;
        // After 10 messages with max_history=5, trim_to=3:
        // trim fires at 6 msgs (keeps 3), then again at 6 (keeps 3),
        // final add brings it to 4.
        assert_eq!(msgs.len(), 4);
        assert_eq!(msgs[0].content, "msg 6");
        assert_eq!(msgs[1].content, "msg 7");
        assert_eq!(msgs[2].content, "msg 8");
        assert_eq!(msgs[3].content, "msg 9");
    }

    #[tokio::test]
    async fn test_cleanup_idle_sessions() {
        let mgr = SessionManager::new(100, 50, Duration::from_millis(100));
        mgr.add_message("s1", MessageRole::User, "hello").await;
        assert_eq!(mgr.session_count().await, 1);

        // Wait for the session to become idle
        tokio::time::sleep(Duration::from_millis(150)).await;

        let removed = mgr.cleanup_idle().await;
        assert_eq!(removed, 1);
        assert_eq!(mgr.session_count().await, 0);
    }

    #[tokio::test]
    async fn test_session_isolation() {
        let mgr = SessionManager::new(100, 50, Duration::from_secs(300));
        mgr.add_message("s1", MessageRole::User, "session one")
            .await;
        mgr.add_message("s2", MessageRole::User, "session two")
            .await;

        let msgs1 = mgr.get_messages("s1").await;
        let msgs2 = mgr.get_messages("s2").await;

        assert_eq!(msgs1.len(), 1);
        assert_eq!(msgs1[0].content, "session one");

        assert_eq!(msgs2.len(), 1);
        assert_eq!(msgs2[0].content, "session two");
    }
}
