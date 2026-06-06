use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Long-term memory storage for context persistence.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct MemoryStore {
    entries: HashMap<String, MemoryEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MemoryEntry {
    pub key: String,
    pub value: String,
    pub tags: Vec<String>,
}

impl MemoryStore {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn store(&mut self, key: impl Into<String>, value: impl Into<String>, tags: Vec<String>) {
        let key = key.into();
        self.entries.insert(
            key.clone(),
            MemoryEntry {
                key,
                value: value.into(),
                tags,
            },
        );
    }

    pub fn retrieve(&self, key: &str) -> Option<&MemoryEntry> {
        self.entries.get(key)
    }

    pub fn remove(&mut self, key: &str) -> Option<MemoryEntry> {
        self.entries.remove(key)
    }

    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_memory_store_and_retrieve() {
        let mut store = MemoryStore::new();
        store.store("key1", "value1", vec!["tag1".into()]);
        let entry = store.retrieve("key1").unwrap();
        assert_eq!(entry.value, "value1");
    }

    #[test]
    fn test_memory_remove() {
        let mut store = MemoryStore::new();
        store.store("key1", "value1", vec![]);
        store.remove("key1");
        assert!(store.retrieve("key1").is_none());
    }
}
