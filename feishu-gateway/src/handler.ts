import { FeishuConnection } from "./feishu.js";
import { DaemonClient } from "./daemon-client.js";
import { StreamingManager } from "./streaming.js";
import type { ChatRequest } from "./types.js";

export class MessageHandler {
  private feishu: FeishuConnection;
  private daemon: DaemonClient;
  private streaming: StreamingManager;

  constructor(feishu: FeishuConnection, daemon: DaemonClient) {
    this.feishu = feishu;
    this.daemon = daemon;
    this.streaming = new StreamingManager(
      feishu.getClient(),
      async (sessionId, content) => {
        console.log(`Fallback reply for ${sessionId}: ${content}`);
      }
    );
  }

  async handleMessage(event: {
    message_id: string;
    chat_id: string;
    chat_type: string;
    content: string;
    sender: { sender_id: { open_id: string } };
  }) {
    const { message_id, chat_id, chat_type, sender } = event;
    const userId = sender.sender_id.open_id;

    let content: string;
    try {
      const parsed = JSON.parse(event.content);
      content = parsed.text || "";
    } catch {
      content = event.content;
    }

    if (chat_type === "group") {
      content = content.replace(/@_user_\d+/g, "").trim();
    }

    if (!content) return;

    const sessionId =
      chat_type === "p2p"
        ? `feishu:${userId}`
        : `feishu:group:${chat_id}:${userId}`;

    const request: ChatRequest = {
      type: "chat_request",
      session_id: sessionId,
      user_id: userId,
      content,
      channel: "feishu",
    };

    try {
      const response = await this.daemon.sendChat(request);

      if (response.error) {
        await this.feishu.replyMessage(message_id, `Error: ${response.error}`);
        return;
      }

      if (response.deltas.length > 0) {
        await this.streaming.onDone(sessionId, chat_id, message_id, response.fullResponse);
      } else {
        await this.feishu.replyMessage(message_id, response.fullResponse);
      }
    } catch (e) {
      console.error("Failed to handle message:", e);
      await this.feishu.replyMessage(message_id, "Sorry, something went wrong.");
    }
  }
}
