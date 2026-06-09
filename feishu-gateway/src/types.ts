// WebSocket protocol types matching golem-daemon JSON schema.

export interface ChatRequest {
  type: "chat_request";
  session_id: string;
  user_id: string;
  content: string;
  channel: "feishu";
}

export interface StreamDelta {
  type: "chat_delta";
  session_id: string;
  delta: string;
}

export interface StreamDone {
  type: "chat_done";
  session_id: string;
  full_response: string;
}

export interface ChatError {
  type: "chat_error";
  session_id: string;
  error: string;
}

export type DaemonMessage = StreamDelta | StreamDone | ChatError;

export function isStreamDelta(msg: DaemonMessage): msg is StreamDelta {
  return msg.type === "chat_delta";
}

export function isStreamDone(msg: DaemonMessage): msg is StreamDone {
  return msg.type === "chat_done";
}

export function isChatError(msg: DaemonMessage): msg is ChatError {
  return msg.type === "chat_error";
}
