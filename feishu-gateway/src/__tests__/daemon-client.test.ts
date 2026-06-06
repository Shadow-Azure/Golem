import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { DaemonClient } from "../daemon-client.js";
import { WebSocket, WebSocketServer } from "ws";

describe("DaemonClient", () => {
  let wss: WebSocketServer;
  let port: number;

  beforeEach(async () => {
    wss = new WebSocketServer({ port: 0 });
    await new Promise<void>((resolve) => {
      wss.on("listening", () => {
        port = (wss.address() as any).port;
        resolve();
      });
    });
  });

  afterEach(() => {
    wss.close();
  });

  it("should connect and receive messages", async () => {
    wss.on("connection", (ws) => {
      ws.on("message", (data) => {
        const msg = JSON.parse(data.toString());
        ws.send(JSON.stringify({
          type: "chat_delta",
          session_id: msg.session_id,
          delta: "echo: " + msg.content,
        }));
        ws.send(JSON.stringify({
          type: "chat_done",
          session_id: msg.session_id,
          full_response: "echo: " + msg.content,
        }));
      });
    });

    const client = new DaemonClient(`ws://localhost:${port}`);
    await client.connect();

    const response = await client.sendChat({
      type: "chat_request",
      session_id: "sess_1",
      user_id: "user_1",
      content: "hello",
      channel: "feishu",
    });

    expect(response.fullResponse).toBe("echo: hello");
    expect(response.deltas).toContain("echo: hello");

    client.disconnect();
  });

  it("should handle connection errors", async () => {
    const client = new DaemonClient("ws://localhost:99999");
    await expect(client.connect()).rejects.toThrow();
  });
});
