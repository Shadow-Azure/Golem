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

  it("should connect and stream deltas", async () => {
    wss.on("connection", (ws) => {
      ws.on("message", (data) => {
        const msg = JSON.parse(data.toString());
        // Send two deltas then done
        ws.send(JSON.stringify({
          type: "chat_delta",
          session_id: msg.session_id,
          delta: "echo: ",
        }));
        ws.send(JSON.stringify({
          type: "chat_delta",
          session_id: msg.session_id,
          delta: msg.content,
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

    const receivedDeltas: string[] = [];

    const response = await client.sendChat(
      {
        type: "chat_request",
        session_id: "sess_1",
        user_id: "user_1",
        content: "hello",
        channel: "feishu",
      },
      (delta) => receivedDeltas.push(delta),
    );

    expect(response.fullResponse).toBe("echo: hello");
    expect(receivedDeltas).toEqual(["echo: ", "hello"]);
    expect(response.deltas).toEqual(["echo: ", "hello"]);

    client.disconnect();
  });

  it("should handle connection errors", async () => {
    const client = new DaemonClient("ws://localhost:99999");
    await expect(client.connect()).rejects.toThrow();
  });

  it("should handle concurrent requests", async () => {
    wss.on("connection", (ws) => {
      ws.on("message", (data) => {
        const msg = JSON.parse(data.toString());
        ws.send(JSON.stringify({
          type: "chat_delta",
          session_id: msg.session_id,
          delta: msg.content,
        }));
        ws.send(JSON.stringify({
          type: "chat_done",
          session_id: msg.session_id,
          full_response: msg.content,
        }));
      });
    });

    const client = new DaemonClient(`ws://localhost:${port}`);
    await client.connect();

    const [r1, r2] = await Promise.all([
      client.sendChat(
        { type: "chat_request", session_id: "sess_1", user_id: "u1", content: "hello", channel: "feishu" },
        () => {},
      ),
      client.sendChat(
        { type: "chat_request", session_id: "sess_2", user_id: "u2", content: "world", channel: "feishu" },
        () => {},
      ),
    ]);

    expect(r1.fullResponse).toBe("hello");
    expect(r2.fullResponse).toBe("world");

    client.disconnect();
  });
});
