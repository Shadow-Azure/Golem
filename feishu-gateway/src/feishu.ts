import * as lark from "@larksuiteoapi/node-sdk";

interface FeishuConfig {
  appId: string;
  appSecret: string;
  verificationToken: string;
}

interface FeishuMessageEvent {
  message_id: string;
  chat_id: string;
  chat_type: "p2p" | "group";
  content: string;
  sender: {
    sender_id: {
      open_id: string;
    };
  };
  mentions?: Array<{
    key: string;
    id: { open_id: string };
  }>;
}

export type MessageHandler = (event: FeishuMessageEvent) => Promise<void>;

export class FeishuConnection {
  private client: lark.Client;
  private wsClient: lark.WSClient | null = null;
  private config: FeishuConfig;
  private handler: MessageHandler | null = null;
  private recentMessageIds: Map<string, number> = new Map();
  private dedupTtl = 5 * 60 * 1000; // 5 minutes

  constructor(config: FeishuConfig) {
    this.config = config;
    this.client = new lark.Client({
      appId: config.appId,
      appSecret: config.appSecret,
    });
  }

  onMessage(handler: MessageHandler) {
    this.handler = handler;
  }

  async connect() {
    this.wsClient = new lark.WSClient({
      appId: this.config.appId,
      appSecret: this.config.appSecret,
    });

    this.wsClient.start({
      eventDispatcher: new lark.EventDispatcher({
        verificationToken: this.config.verificationToken,
      }).register({
        "im.message.receive_v1": async (data: any) => {
          const event = data.event as FeishuMessageEvent;

          if (this.isDuplicate(event.message_id)) {
            return;
          }

          if (event.chat_type === "group") {
            const isMentioned =
              event.mentions?.some(
                (m) => m.id.open_id === this.client.appId
              ) ?? false;
            if (!isMentioned) return;
          }

          if (this.handler) {
            await this.handler(event);
          }
        },
      }),
    });

    console.log("Feishu WebSocket connected");
  }

  private isDuplicate(messageId: string): boolean {
    this.cleanupExpired();
    if (this.recentMessageIds.has(messageId)) {
      return true;
    }
    this.recentMessageIds.set(messageId, Date.now());
    return false;
  }

  private cleanupExpired() {
    const now = Date.now();
    for (const [id, timestamp] of this.recentMessageIds) {
      if (now - timestamp > this.dedupTtl) {
        this.recentMessageIds.delete(id);
      }
    }
  }

  async sendMessage(chatId: string, content: string) {
    await this.client.im.message.create({
      params: { receive_id_type: "chat_id" },
      data: {
        receive_id: chatId,
        msg_type: "text",
        content: JSON.stringify({ text: content }),
      },
    });
  }

  async replyMessage(messageId: string, content: string) {
    await this.client.im.message.reply({
      path: { message_id: messageId },
      data: {
        msg_type: "text",
        content: JSON.stringify({ text: content }),
      },
    });
  }

  getClient(): lark.Client {
    return this.client;
  }
}
