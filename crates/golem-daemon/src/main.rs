use anyhow::Result;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    tracing::info!("Golem daemon v{} starting", env!("CARGO_PKG_VERSION"));
    Ok(())
}
