import WebSocket from "ws";
import type { ChatRequest, DaemonMessage } from "./types.js";
import { isStreamDelta, isStreamDone, isChatError } from "./types.js";

export interface ChatResponse {
  deltas: string[];
  fullResponse: string;
  error?: string;
}

type MessageCallback = (msg: DaemonMessage) => void;

export class DaemonClient {
  private ws: WebSocket | null = null;
  private url: string;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectBaseDelay = 1000;
  private sessionCallbacks: Map<string, MessageCallback> = new Map();

  constructor(url: string) {
    this.url = url;
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(this.url);

      ws.on("open", () => {
        this.reconnectAttempts = 0;
        resolve();
      });

      ws.on("error", (err) => {
        if (ws.readyState === WebSocket.CONNECTING) {
          reject(err);
        }
      });

      ws.on("close", () => {
        this.ws = null;
        this.scheduleReconnect();
      });

      ws.on("message", (data: WebSocket.Data) => {
        try {
          const msg: DaemonMessage = JSON.parse(data.toString());
          const sessionId = msg.session_id;
          const callback = this.sessionCallbacks.get(sessionId);
          if (callback) {
            callback(msg);
          }
        } catch (e) {
          console.error("Failed to parse daemon message:", e);
        }
      });

      this.ws = ws;
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

  /**
   * Send a chat request and receive streaming deltas via callbacks.
   * onDelta is called for each streaming token, onDone when complete.
   */
  sendChat(
    request: ChatRequest,
    onDelta: (delta: string) => void,
  ): Promise<ChatResponse> {
    return new Promise((resolve, reject) => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        reject(new Error("Not connected to daemon"));
        return;
      }

      const deltas: string[] = [];
      let fullResponse = "";

      const timeout = setTimeout(() => {
        this.sessionCallbacks.delete(request.session_id);
        reject(new Error("Chat request timed out"));
      }, 60_000);

      const handler = (msg: DaemonMessage) => {
        if (isStreamDelta(msg)) {
          deltas.push(msg.delta);
          onDelta(msg.delta);
        } else if (isStreamDone(msg)) {
          fullResponse = msg.full_response;
          clearTimeout(timeout);
          this.sessionCallbacks.delete(request.session_id);
          resolve({ deltas, fullResponse });
        } else if (isChatError(msg)) {
          clearTimeout(timeout);
          this.sessionCallbacks.delete(request.session_id);
          resolve({ deltas, fullResponse, error: msg.error });
        }
      };

      this.sessionCallbacks.set(request.session_id, handler);
      this.ws.send(JSON.stringify(request));
    });
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.sessionCallbacks.clear();
  }
}
