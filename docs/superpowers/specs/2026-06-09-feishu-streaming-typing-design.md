# Feishu Streaming Reply & Typing Indicator Design

> **Date**: 2026-06-09
> **Status**: Draft
> **Scope**: Go Feishu channel plugin (`plugins/channels/feishu/`)

---

## Problem

Golem 的飞书插件目前存在两个体验问题：

1. **无流式回复**：用户发送消息后，需要等待 LLM 完全生成回复才能看到内容（可能 10-30 秒无反馈）
2. **无打字指示器**：用户无法知道 bot 是否正在处理消息

对比 openclaw 的飞书集成，用户体验差距明显：

| 特性 | Golem 当前 | openclaw | 目标 |
|------|-----------|----------|------|
| 流式回复 | ❌ 一次性发送 | ✅ 流式卡片 | ✅ 流式卡片 |
| 打字指示器 | ❌ 无 | ✅ Typing emoji reaction | ✅ Typing emoji reaction |

## Solution

### 1. 接口层扩展

在 `internal/plugin/interfaces.go` 中新增两个可选能力接口，通过类型断言检测 channel 是否支持。

#### 1.1 StreamingCapable 接口

```go
// StreamingCapable is an optional interface for channels that support
// streaming reply output (e.g., Feishu Card Kit).
type StreamingCapable interface {
    // CreateStreamReply creates a streaming reply session.
    // Returns a StreamSession for subsequent delta updates.
    CreateStreamReply(sessionID string, opts StreamReplyOptions) (*StreamSession, error)

    // SendDelta sends a streaming delta update to the reply session.
    SendDelta(session *StreamSession, delta string) error

    // FinishStream completes the streaming reply.
    FinishStream(session *StreamSession) error
}

// StreamReplyOptions contains options for creating a streaming reply.
type StreamReplyOptions struct {
    MessageID string // The message ID to reply to
    ChatID    string // The chat/session ID
}

// StreamSession holds state for an active streaming reply.
type StreamSession struct {
    SessionID string
    CardID    string // Feishu card message ID for subsequent updates
}
```

#### 1.2 TypingCapable 接口

```go
// TypingCapable is an optional interface for channels that support
// typing indicators (e.g., Feishu emoji reactions).
type TypingCapable interface {
    // StartTyping begins showing a typing indicator for the given message.
    StartTyping(sessionID string, messageID string) error

    // StopTyping stops showing the typing indicator.
    StopTyping(sessionID string) error
}
```

#### 1.3 能力检测模式

在 Engine 层通过类型断言检测 channel 是否支持这些能力：

```go
if tc, ok := channel.(TypingCapable); ok {
    tc.StartTyping(sessionID, messageID)
    defer tc.StopTyping(sessionID)
}
```

### 2. 打字指示器实现

在 `plugins/channels/feishu/typing.go` 中实现 `TypingCapable` 接口。

#### 2.1 核心实现

- **StartTyping**: 调用飞书 API `POST /open-apis/im/v1/messages/{message_id}/reactions`，添加 `"Typing"` emoji reaction
- **StopTyping**: 调用飞书 API `DELETE /open-apis/im/v1/messages/{message_id}/reactions/{reaction_id}`，删除 reaction

#### 2.2 防护机制

| 机制 | 说明 | 实现位置 |
|------|------|---------|
| 消息年龄过滤 | 超过 2 分钟的消息不添加打字指示器 | `typing.go` |
| 去重 | reaction 已存在时不重复添加 | `typing.go` |
| 限流检测 | 检测飞书限流码 (99991400/99991403/429) | `typing.go` |
| TTL 超时 | 60 秒后自动移除，防止泄漏 | `typing.go` |
| 断路器 | 连续 2 次失败后停止尝试 | `typing.go` |

#### 2.3 状态管理

```go
type TypingState struct {
    MessageID  string
    ReactionID string
}

type TypingManager struct {
    mu     sync.Mutex
    states map[string]*TypingState // sessionID -> state
}
```

### 3. 流式卡片实现

在 `plugins/channels/feishu/streaming.go` 中实现 `StreamingCapable` 接口。

参考飞书 Gateway 的 `StreamingManager`（`feishu-gateway/src/streaming.ts`）的设计。

#### 3.1 核心实现

- **CreateStreamReply**: 创建飞书 Interactive Card，初始内容为加载状态
- **SendDelta**: 更新卡片内容，带节流控制
- **FinishStream**: 发送最终版本的卡片

#### 3.2 节流策略

```go
const (
    minUpdateInterval = 160 * time.Millisecond // 最小更新间隔
    minCharsDelta     = 18                       // 最小字符变更量
)
```

- 每次 `SendDelta` 检查距上次更新是否超过 `minUpdateInterval`
- 检查累积的字符变更是否超过 `minCharsDelta`
- 不满足条件时缓存 delta，延迟发送

#### 3.3 卡片模板

```json
{
  "config": { "wide_screen_mode": true },
  "header": {
    "title": { "tag": "plain_text", "content": "Golem" },
    "template": "blue"
  },
  "elements": [
    {
      "tag": "markdown",
      "content": "<streaming content>"
    }
  ]
}
```

### 4. Engine 层集成

修改 `internal/core/engine.go` 中的消息处理流程：

```go
func (e *Engine) processMessage(sessionID, channelType, messageID, content string) {
    channel := e.pluginManager.GetChannel(channelType)
    provider := e.getProvider()
    messages := e.sessionManager.GetHistory(sessionID)

    // Phase 1: Start typing indicator
    if tc, ok := channel.(TypingCapable); ok {
        if err := tc.StartTyping(sessionID, messageID); err != nil {
            log.Printf("failed to start typing: %v", err)
        }
        defer func() {
            if err := tc.StopTyping(sessionID); err != nil {
                log.Printf("failed to stop typing: %v", err)
            }
        }()
    }

    // Phase 2: Stream or non-stream reply
    if sc, ok := channel.(StreamingCapable); ok && provider.SupportsStreaming() {
        e.handleStreamingReply(sc, provider, sessionID, messageID, content, messages)
    } else {
        e.handleNonStreamingReply(channel, provider, sessionID, content, messages)
    }
}

func (e *Engine) handleStreamingReply(
    sc StreamingCapable,
    provider ProviderPlugin,
    sessionID, messageID, content string,
    messages []Message,
) {
    session, err := sc.CreateStreamReply(sessionID, StreamReplyOptions{
        MessageID: messageID,
    })
    if err != nil {
        log.Printf("failed to create stream reply: %v", err)
        return
    }

    ctx := context.Background()
    fullResponse := ""

    err = provider.ChatStream(ctx, messages, func(delta string) {
        fullResponse += delta
        if err := sc.SendDelta(session, delta); err != nil {
            log.Printf("failed to send delta: %v", err)
        }
    })

    if err != nil {
        log.Printf("stream error: %v", err)
    }

    if err := sc.FinishStream(session); err != nil {
        log.Printf("failed to finish stream: %v", err)
    }

    // Save assistant message to session
    e.sessionManager.AddMessage(sessionID, "assistant", fullResponse)
}
```

### 5. 错误处理

| 场景 | 处理方式 |
|------|---------|
| 飞书 API 限流 | 打字指示器停止重试，流式降级为非流式 |
| 飞书 API 超时 | 重试 1 次，失败后静默降级 |
| LLM 流式中断 | FinishStream 发送已生成的部分内容 |
| 卡片创建失败 | 降级为普通文本消息 |
| reaction 添加失败 | 静默忽略，不影响回复 |

### 6. 配置项

在 `configs/config.yaml` 中新增：

```yaml
plugins:
  channels:
    feishu:
      typing_indicator: true     # 是否启用打字指示器（默认 true）
      streaming: true            # 是否启用流式回复（默认 true）
      stream_throttle_ms: 160    # 流式更新最小间隔（默认 160ms）
```

### 7. 测试策略

| 层级 | 文件 | 覆盖内容 |
|------|------|---------|
| **UT** | `typing_test.go` | 添加/删除 reaction、去重、限流检测、TTL 超时、消息年龄过滤 |
| **UT** | `streaming_test.go` | 卡片创建/更新/完成、节流逻辑、降级处理 |
| **UT** | `engine_test.go` | 流式/非流式流程编排、TypingCapable/StreamingCapable 检测 |
| **IT** | `plugin_test.go` | 完整消息→流式回复→打字指示器生命周期 |

### 8. 文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/plugin/interfaces.go` | 修改 | 新增 StreamingCapable、TypingCapable 接口定义 |
| `plugins/channels/feishu/typing.go` | 新增 | 打字指示器实现 |
| `plugins/channels/feishu/typing_test.go` | 新增 | 打字指示器测试 |
| `plugins/channels/feishu/streaming.go` | 新增 | 流式卡片实现 |
| `plugins/channels/feishu/streaming_test.go` | 新增 | 流式卡片测试 |
| `plugins/channels/feishu/plugin.go` | 修改 | 集成打字指示器和流式回复到消息处理流程 |
| `internal/core/engine.go` | 修改 | 添加 StreamingCapable/TypingCapable 检测和流程编排 |
| `internal/core/engine_test.go` | 修改 | 新增流式回复相关测试 |
| `configs/config.yaml` | 修改 | 新增 typing_indicator、streaming 配置项 |
| `internal/config/types.go` | 修改 | 新增配置字段 |

---

## References

- 飞书 Reaction API: https://open.feishu.cn/document/server-docs/im-v1/message-reaction/emojis-introduce
- 飞书 Card Kit: https://open.feishu.cn/document/uAjLw4CM/ukzMukzMukzM/feishu-cards/quick-start
- openclaw 飞书 typing 实现: `extensions/feishu/src/typing.ts`
- openclaw 飞书 streaming 实现: `extensions/feishu/src/streaming-card.ts`
- Golem 飞书 Gateway StreamingManager: `feishu-gateway/src/streaming.ts`
