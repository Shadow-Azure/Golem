import "dotenv/config";
import { FeishuConnection } from "./feishu.js";
import { DaemonClient } from "./daemon-client.js";
import { MessageHandler } from "./handler.js";

async function main() {
  const feishuAppId = process.env.FEISHU_APP_ID;
  const feishuAppSecret = process.env.FEISHU_APP_SECRET;
  const feishuVerificationToken = process.env.FEISHU_VERIFICATION_TOKEN;
  const daemonWsUrl = process.env.DAEMON_WS_URL || "ws://localhost:9921/ws";

  if (!feishuAppId || !feishuAppSecret || !feishuVerificationToken) {
    console.error(
      "Missing required env vars: FEISHU_APP_ID, FEISHU_APP_SECRET, FEISHU_VERIFICATION_TOKEN"
    );
    process.exit(1);
  }

  const daemon = new DaemonClient(daemonWsUrl);
  try {
    await daemon.connect();
    console.log(`Connected to daemon at ${daemonWsUrl}`);
  } catch (e) {
    console.error("Failed to connect to daemon:", e);
    process.exit(1);
  }

  const feishu = new FeishuConnection({
    appId: feishuAppId,
    appSecret: feishuAppSecret,
    verificationToken: feishuVerificationToken,
  });

  const handler = new MessageHandler(feishu, daemon);
  feishu.onMessage(async (event) => {
    await handler.handleMessage(event);
  });

  await feishu.connect();
  console.log("Feishu gateway started");

  const shutdown = () => {
    console.log("Shutting down...");
    daemon.disconnect();
    process.exit(0);
  };

  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}

main().catch((e) => {
  console.error("Fatal error:", e);
  process.exit(1);
});
