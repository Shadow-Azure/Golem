import WebSocket from "ws";
import type { ChatRequest, DaemonMessage } from "./types.js";
import { isStreamDelta, isStreamDone, isChatError } from "./types.js";

interface ChatResponse {
  deltas: string[];
  fullResponse: string;
  error?: string;
}

export class DaemonClient {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectBaseDelay = 1000;

  constructor(url: string) {
    this.url = url;
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url);

      this.ws.on("open", () => {
        this.reconnectAttempts = 0;
        resolve();
      });

      this.ws.on("error", (err) => {
        if (this.ws?.readyState === WebSocket.CONNECTING) {
          reject(err);
        }
      });

      this.ws.on("close", () => {
        this.scheduleReconnect();
      });
    });
  }

  private scheduleReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error("Max reconnect attempts reached");
      return;
    }

    const delay = this.reconnectBaseDelay * Math.pow(2, this.reconnectAttempts);
    this.reconnectAttempts++;

    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
    setTimeout(() => {
      this.connect().catch((err) => {
        console.error("Reconnect failed:", err.message);
      });
    }, delay);
  }

  sendChat(request: ChatRequest): Promise<ChatResponse> {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        reject(new Error("Not connected to daemon"));
        return;
      }

      const deltas: string[] = [];
      let fullResponse = "";

      const handler = (data: WebSocket.Data) => {
        const msg: DaemonMessage = JSON.parse(data.toString());
        if (msg.session_id !== request.session_id) return;

        if (isStreamDelta(msg)) {
          deltas.push(msg.delta);
        } else if (isStreamDone(msg)) {
          fullResponse = msg.full_response;
          this.ws?.removeListener("message", handler);
          resolve({ deltas, fullResponse });
        } else if (isChatError(msg)) {
          this.ws?.removeListener("message", handler);
          resolve({ deltas, fullResponse, error: msg.error });
        }
      };

      this.ws.on("message", handler);
      this.ws.send(JSON.stringify(request));

      setTimeout(() => {
        this.ws?.removeListener("message", handler);
        reject(new Error("Chat request timed out"));
      }, 60_000);
    });
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}
