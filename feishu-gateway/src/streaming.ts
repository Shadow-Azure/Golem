import type { Client } from "@larksuiteoapi/node-sdk";

interface StreamingCard {
  cardId: string;
  content: string;
  lastUpdateTime: number;
  minInterval: number;
  minChars: number;
}

export class StreamingManager {
  private cards: Map<string, StreamingCard> = new Map();
  private client: Client;
  private fallbackHandler: (sessionId: string, content: string) => Promise<void>;

  constructor(
    client: Client,
    fallbackHandler: (sessionId: string, content: string) => Promise<void>
  ) {
    this.client = client;
    this.fallbackHandler = fallbackHandler;
  }

  async onDelta(sessionId: string, chatId: string, messageId: string, delta: string) {
    let card = this.cards.get(sessionId);

    if (!card) {
      try {
        card = await this.createCard(chatId, messageId);
        this.cards.set(sessionId, card);
      } catch (e) {
        console.error("Failed to create streaming card:", e);
        return;
      }
    }

    card.content += delta;

    const now = Date.now();
    if (
      now - card.lastUpdateTime >= card.minInterval &&
      card.content.length >= card.minChars
    ) {
      await this.updateCard(card);
      card.lastUpdateTime = now;
    }
  }

  async onDone(sessionId: string, chatId: string, messageId: string, fullResponse: string) {
    const card = this.cards.get(sessionId);

    if (card) {
      card.content = fullResponse;
      await this.updateCard(card);
      await this.closeCard(card);
      this.cards.delete(sessionId);
    } else {
      await this.fallbackHandler(sessionId, fullResponse);
    }
  }

  private async createCard(chatId: string, messageId: string): Promise<StreamingCard> {
    const response = await (this.client as any).cardkit.v1.card.create({
      data: {
        type: "card_kit",
        data: {
          card_link: {
            url: `https://open.feishu.cn/open-apis/cardkit/v1/cards/{card_id}/elements/content/content`,
          },
          elements: [
            {
              tag: "markdown",
              content: "",
              element_id: "content",
            },
          ],
        },
      },
    });

    return {
      cardId: response.card_id || "",
      content: "",
      lastUpdateTime: Date.now(),
      minInterval: 160,
      minChars: 18,
    };
  }

  private async updateCard(card: StreamingCard) {
    try {
      await (this.client as any).cardkit.v1.cardElement.update({
        path: { card_id: card.cardId, element_id: "content" },
        data: {
          content: card.content,
        },
      });
    } catch (e) {
      console.error("Failed to update card:", e);
    }
  }

  private async closeCard(card: StreamingCard) {
    try {
      await (this.client as any).cardkit.v1.card.patch({
        path: { card_id: card.cardId },
        data: {
          type: "card_kit",
          data: {
            elements: [
              {
                tag: "markdown",
                content: card.content,
                element_id: "content",
              },
            ],
          },
        },
      });
    } catch (e) {
      console.error("Failed to close card:", e);
    }
  }
}
