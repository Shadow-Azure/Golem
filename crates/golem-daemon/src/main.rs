use anyhow::Result;
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
                    temperature: 0.7,
                    max_tokens: 4096,
                    system_prompt: None,
                },
                session: config::SessionConfig::default(),
            }
        }
    };

    // Create LLM provider
    let llm: Arc<dyn LlmProvider> = Arc::new(openai::OpenAiProvider::new(
        app_config.llm.base_url.clone(),
        app_config.llm.api_key.clone(),
    )?);

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

    // Graceful shutdown: ctrl-c triggers axum's drain
    let server = axum::serve(listener, app).with_graceful_shutdown(async {
        tokio::signal::ctrl_c().await.ok();
        tracing::info!("Received shutdown signal, draining connections...");
    });

    if let Err(e) = server.await {
        tracing::error!("Server error: {e}");
    }

    tracing::info!("Golem daemon stopped");
    Ok(())
}
