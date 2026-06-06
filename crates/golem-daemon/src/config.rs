use anyhow::{Context, Result};
use serde::Deserialize;
use std::fs;
use std::path::{Path, PathBuf};

/// Top-level application configuration loaded from `golem.toml`.
#[derive(Debug, Default, Deserialize)]
#[serde(default)]
#[allow(dead_code)]
pub struct AppConfig {
    pub server: ServerConfig,
    pub llm: LlmConfig,
    pub session: SessionConfig,
}

/// HTTP server configuration.
#[derive(Debug, Deserialize)]
#[serde(default)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
}

/// LLM provider configuration.
#[derive(Debug, Deserialize)]
#[serde(default)]
pub struct LlmConfig {
    pub provider: String,
    pub model: String,
    pub api_key: String,
    pub base_url: String,
    pub temperature: f32,
    pub max_tokens: u32,
}

/// Session management configuration.
#[derive(Debug, Deserialize)]
#[serde(default)]
pub struct SessionConfig {
    pub max_history: usize,
    pub trim_to: usize,
    pub idle_timeout_minutes: u64,
}

impl Default for ServerConfig {
    fn default() -> Self {
        Self {
            host: "0.0.0.0".to_string(),
            port: 9921,
        }
    }
}

impl Default for LlmConfig {
    fn default() -> Self {
        Self {
            provider: "openai".to_string(),
            model: "gpt-4o".to_string(),
            api_key: String::new(),
            base_url: "https://api.openai.com/v1".to_string(),
            temperature: 0.7,
            max_tokens: 4096,
        }
    }
}

impl Default for SessionConfig {
    fn default() -> Self {
        Self {
            max_history: 50,
            trim_to: 20,
            idle_timeout_minutes: 30,
        }
    }
}

#[allow(dead_code)]
impl AppConfig {
    /// Parse a config file from the given path.
    pub fn from_file(path: &Path) -> Result<Self> {
        let content = fs::read_to_string(path)
            .with_context(|| format!("failed to read config file: {}", path.display()))?;
        let config: AppConfig = toml::from_str(&content)
            .with_context(|| format!("failed to parse config file: {}", path.display()))?;
        Ok(config)
    }

    /// Load configuration from one of the standard locations.
    ///
    /// Search order:
    /// 1. `./golem.toml` (current directory)
    /// 2. `~/.config/golem/golem.toml` (XDG-style)
    ///
    /// If no file is found, returns defaults.
    pub fn load() -> Result<Self> {
        // 1. Current directory
        let local = Path::new("golem.toml");
        if local.exists() {
            tracing::info!("loading config from {}", local.display());
            return Self::from_file(local);
        }

        // 2. XDG config directory
        if let Some(config_dir) = dirs::config_dir() {
            let xdg = config_dir.join("golem").join("golem.toml");
            if xdg.exists() {
                tracing::info!("loading config from {}", xdg.display());
                return Self::from_file(&xdg);
            }
        }

        tracing::info!("no config file found, using defaults");
        Ok(Self::default())
    }

    /// Return the default XDG config path for informational purposes.
    pub fn default_config_path() -> Option<PathBuf> {
        dirs::config_dir().map(|d| d.join("golem").join("golem.toml"))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::NamedTempFile;

    #[test]
    fn test_parse_full_config() {
        let toml_content = r#"
[server]
host = "127.0.0.1"
port = 8080

[llm]
provider = "anthropic"
model = "claude-sonnet-4-20250514"
api_key = "sk-test-key"
base_url = "https://api.anthropic.com/v1"
temperature = 0.5
max_tokens = 8192

[session]
max_history = 100
trim_to = 40
idle_timeout_minutes = 60
"#;
        let config: AppConfig = toml::from_str(toml_content).unwrap();
        assert_eq!(config.server.host, "127.0.0.1");
        assert_eq!(config.server.port, 8080);
        assert_eq!(config.llm.provider, "anthropic");
        assert_eq!(config.llm.model, "claude-sonnet-4-20250514");
        assert_eq!(config.llm.api_key, "sk-test-key");
        assert_eq!(config.llm.base_url, "https://api.anthropic.com/v1");
        assert!((config.llm.temperature - 0.5).abs() < f32::EPSILON);
        assert_eq!(config.llm.max_tokens, 8192);
        assert_eq!(config.session.max_history, 100);
        assert_eq!(config.session.trim_to, 40);
        assert_eq!(config.session.idle_timeout_minutes, 60);
    }

    #[test]
    fn test_parse_minimal_config() {
        let toml_content = r#"
[llm]
api_key = "sk-minimal"
"#;
        let config: AppConfig = toml::from_str(toml_content).unwrap();
        // Provided value
        assert_eq!(config.llm.api_key, "sk-minimal");
        // All other fields should be defaults
        assert_eq!(config.server.host, "0.0.0.0");
        assert_eq!(config.server.port, 9921);
        assert_eq!(config.llm.provider, "openai");
        assert_eq!(config.llm.model, "gpt-4o");
        assert_eq!(config.llm.base_url, "https://api.openai.com/v1");
        assert!((config.llm.temperature - 0.7).abs() < f32::EPSILON);
        assert_eq!(config.llm.max_tokens, 4096);
        assert_eq!(config.session.max_history, 50);
        assert_eq!(config.session.trim_to, 20);
        assert_eq!(config.session.idle_timeout_minutes, 30);
    }

    #[test]
    fn test_from_file() {
        let toml_content = r#"
[server]
port = 3000

[llm]
api_key = "sk-file-test"
model = "gpt-4o-mini"

[session]
max_history = 25
"#;
        let mut tmp = NamedTempFile::new().unwrap();
        write!(tmp, "{}", toml_content).unwrap();
        let path = tmp.path();

        let config = AppConfig::from_file(path).unwrap();
        assert_eq!(config.server.port, 3000);
        assert_eq!(config.server.host, "0.0.0.0"); // default
        assert_eq!(config.llm.api_key, "sk-file-test");
        assert_eq!(config.llm.model, "gpt-4o-mini");
        assert_eq!(config.session.max_history, 25);
        assert_eq!(config.session.trim_to, 20); // default
    }
}
