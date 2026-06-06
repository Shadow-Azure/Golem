use anyhow::Result;

fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    println!("Golem CLI v{}", env!("CARGO_PKG_VERSION"));
    Ok(())
}
