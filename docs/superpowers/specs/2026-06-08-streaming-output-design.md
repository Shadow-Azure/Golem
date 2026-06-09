# Streaming Output Design Specification

**Date**: 2026-06-08
**Status**: Draft
**Author**: Claude Code (Brainstorming)

## 1. Overview

### 1.1 Project Goal

Add streaming output support to all channels:
- CLI: Character-by-character output (typewriter effect)
- WebChat: Server-Sent Events (SSE)
- Feishu: Card Kit streaming updates

### 1.2 Key Design Principles

1. **Unified Architecture**: Same streaming infrastructure for all channels
2. **Provider Agnostic**: Works with OpenAI, Claude, MiniMax
3. **Non-Blocking**: Streaming doesn't block other operations
4. **Error Resilient**: Graceful handling of stream errors

### 1.3 Technology Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| CLI Output | fmt.Print (Go stdlib) | - |
| WebChat | Server-Sent Events | - |
| Feishu | Card Kit API | - |
| Streaming | Go channels | - |

## 2. Architecture

### 2.1 System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Streaming Architecture                   │
├─────────────────────────────────────────────────────────────┤
│  LLM Provider                                               │
│  ├─ OpenAI ChatStream()                                     │
│  ├─ Claude ChatStream()                                     │
│  └─ MiniMax ChatStream()                                    │
│         │                                                   │
│         │ <-chan core.StreamChunk                            │
│         ▼                                                   │
│  ┌─────────────┐                                            │
│  │  EventBus   │ ← EventStreamDelta                         │
│  └─────────────┘                                            │
│         │                                                   │
│    ┌────┴────┬────────┐                                     │
│    ▼         ▼        ▼                                     │
│  CLI      WebChat    飞书                                   │
│ (逐字)    (SSE)    (Card Kit)                               │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Data Flow

```
1. User sends message
2. ChannelPlugin receives message
3. Engine calls ProviderPlugin.ChatStream()
4. Provider returns <-chan StreamChunk
5. ChannelPlugin reads chunks from channel
6. ChannelPlugin outputs chunks to user:
   - CLI: fmt.Print(chunk.Content)
   - WebChat: SSE event
   - Feishu: Card Kit update
7. Stream ends when chunk.Done == true
```

## 3. CLI Streaming

### 3.1 Implementation

```go
// cmd/golem/chat.go

func streamResponse(provider plugin.ProviderPlugin, session *core.Session) (string, error) {
    history, _ := engine.GetSessionManager().GetHistory(session.ID, 50)

    stream, err := provider.ChatStream(context.Background(), history, core.ChatConfig{
        Stream: true,
    })
    if err != nil {
        return "", err
    }

    var fullResponse string
    for chunk := range stream {
        if chunk.Error != nil {
            return fullResponse, chunk.Error
        }
        if chunk.Done {
            break
        }

        // Character-by-character output
        for _, char := range chunk.Content {
            fmt.Print(string(char))
            time.Sleep(20 * time.Millisecond) // Typewriter effect
        }

        fullResponse += chunk.Content
    }

    fmt.Println()
    return fullResponse, nil
}
```

### 3.2 CLI Output Example

```
You: 你好
AI: 你|  ← 逐字显示
AI: 你好|
AI: 你好！|
AI: 你好！有|
AI: 你好！有什么|
AI: 你好！有什么可|
AI: 你好！有什么可以|
AI: 你好！有什么可以帮|
AI: 你好！有什么可以帮你|
AI: 你好！有什么可以帮你吗|
AI: 你好！有什么可以帮你吗？|
```

## 4. WebChat Streaming

### 4.1 SSE Endpoint

```go
// web/api.go

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    // Get message from query params
    message := r.URL.Query().Get("message")
    if message == "" {
        http.Error(w, "Message required", http.StatusBadRequest)
        return
    }

    // Get or create session
    session, err := s.getOrCreateSession()
    if err != nil {
        http.Error(w, "Failed to get session", http.StatusInternalServerError)
        return
    }

    // Add user message
    s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
        Role:    "user",
        Content: message,
    })

    // Get history
    history, _ := s.engine.GetSessionManager().GetHistory(session.ID, 50)

    // Call LLM with streaming
    stream, err := s.provider.ChatStream(r.Context(), history, core.ChatConfig{
        Stream: true,
    })
    if err != nil {
        http.Error(w, "Failed to get response", http.StatusInternalServerError)
        return
    }

    // Stream response
    var fullResponse string
    for chunk := range stream {
        if chunk.Error != nil {
            fmt.Fprintf(w, "data: {\"error\": \"%s\"}\n\n", chunk.Error.Error())
            break
        }
        if chunk.Done {
            fmt.Fprintf(w, "data: {\"done\": true}\n\n")
            break
        }

        // Send chunk
        data, _ := json.Marshal(map[string]string{
            "content": chunk.Content,
        })
        fmt.Fprintf(w, "data: %s\n\n", data)

        fullResponse += chunk.Content

        // Flush if possible
        if f, ok := w.(http.Flusher); ok {
            f.Flush()
        }
    }

    // Add assistant message to session
    s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
        Role:    "assistant",
        Content: fullResponse,
    })
}
```

### 4.2 Vue Frontend

```vue
<!-- ui/src/components/Chat.vue -->

<script setup>
const streamingContent = ref('')
const isStreaming = ref(false)

const sendMessageStream = async () => {
  if (!input.value.trim() || isStreaming.value) return

  const userMessage = input.value.trim()
  input.value = ''
  isStreaming.value = true
  streamingContent.value = ''

  // Add user message
  messages.value.push({ role: 'user', content: userMessage })

  try {
    const eventSource = new EventSource(`/api/stream?message=${encodeURIComponent(userMessage)}`)

    eventSource.onmessage = (event) => {
      const data = JSON.parse(event.data)

      if (data.error) {
        messages.value.push({ role: 'assistant', content: `Error: ${data.error}` })
        eventSource.close()
        isStreaming.value = false
        return
      }

      if (data.done) {
        // Add final message
        if (streamingContent.value) {
          messages.value.push({ role: 'assistant', content: streamingContent.value })
        }
        streamingContent.value = ''
        eventSource.close()
        isStreaming.value = false
        return
      }

      // Append content
      streamingContent.value += data.content
    }

    eventSource.onerror = () => {
      messages.value.push({ role: 'assistant', content: 'Error: Connection lost' })
      streamingContent.value = ''
      isStreaming.value = false
      eventSource.close()
    }
  } catch (error) {
    messages.value.push({ role: 'assistant', content: `Error: ${error.message}` })
    isStreaming.value = false
  }
}
</script>

<template>
  <div class="chat-container">
    <div class="messages">
      <div v-for="msg in messages" :key="msg.id" :class="['message', msg.role]">
        <div class="content">{{ msg.content }}</div>
      </div>
      <div v-if="streamingContent" class="message assistant streaming">
        <div class="content">{{ streamingContent }}<span class="cursor">|</span></div>
      </div>
    </div>

    <div class="input-area">
      <input v-model="input" @keyup.enter="sendMessageStream" placeholder="输入消息..." />
      <button @click="sendMessageStream" :disabled="isStreaming">
        {{ isStreaming ? '发送中...' : '发送' }}
      </button>
    </div>
  </div>
</template>
```

## 5. Feishu Streaming

### 5.1 Card Kit Implementation

```go
// plugins/channels/feishu/streaming.go

type StreamingManager struct {
    client *lark.Client
    logger *slog.Logger
}

func (sm *StreamingManager) SendStreamingMessage(chatID string, stream <-chan core.StreamChunk) error {
    // Create initial card
    cardID, err := sm.createCard(chatID, "")
    if err != nil {
        return err
    }

    var fullContent string
    throttle := time.NewTicker(160 * time.Millisecond) // 160ms throttle
    defer throttle.Stop()

    for chunk := range stream {
        if chunk.Error != nil {
            sm.updateCard(cardID, fullContent+"\n\n[Error: "+chunk.Error.Error()+"]")
            return chunk.Error
        }
        if chunk.Done {
            break
        }

        fullContent += chunk.Content

        // Throttle updates
        select {
        case <-throttle.C:
            sm.updateCard(cardID, fullContent)
        default:
        }
    }

    // Final update
    sm.updateCard(cardID, fullContent)
    return nil
}

func (sm *StreamingManager) createCard(chatID, content string) (string, error) {
    // Create Card Kit message
    card := map[string]interface{}{
        "config": map[string]interface{}{
            "wide_screen_mode": true,
        },
        "elements": []map[string]interface{}{
            {
                "tag": "markdown",
                "content": content + "▌", // Cursor
            },
        },
    }

    // Send card message
    // Return card ID for updates
    return "", nil
}

func (sm *StreamingManager) updateCard(cardID, content string) error {
    // Update card content
    return nil
}
```

### 5.2 Feishu Output Example

```
[Initial Card]
┌─────────────────────────────┐
│ 你好|                        │
└─────────────────────────────┘

[Update 1]
┌─────────────────────────────┐
│ 你好！|                      │
└─────────────────────────────┘

[Update 2]
┌─────────────────────────────┐
│ 你好！有什么可以帮你吗？|     │
└─────────────────────────────┘

[Final]
┌─────────────────────────────┐
│ 你好！有什么可以帮你吗？      │
└─────────────────────────────┘
```

## 6. Provider Integration

### 6.1 OpenAI Provider

```go
// plugins/providers/openai/provider.go

func (p *Provider) ChatStream(ctx context.Context, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error) {
    // Already implemented
    // Returns <-chan core.StreamChunk
}
```

### 6.2 Claude Provider

```go
// plugins/providers/claude/provider.go

func (p *Provider) ChatStream(ctx context.Context, messages []core.Message, config core.ChatConfig) (<-chan core.StreamChunk, error) {
    // Need to implement
    // Call Anthropic streaming API
    // Parse SSE response
    // Return <-chan core.StreamChunk
}
```

### 6.3 MiniMax Provider

```go
// Uses OpenAI-compatible API, so same as OpenAI implementation
```

## 7. Error Handling

### 7.1 Stream Errors

```go
type StreamChunk struct {
    Content string `json:"content"`
    Done    bool   `json:"done"`
    Error   error  `json:"-"`
}
```

### 7.2 Error Handling Strategy

| Error Type | CLI | WebChat | Feishu |
|------------|-----|---------|--------|
| Provider Error | Print error message | SSE error event | Card error message |
| Connection Lost | Print error | Reconnect attempt | Retry with new card |
| Timeout | Print timeout message | SSE timeout event | Card timeout message |

## 8. Testing

### 8.1 CLI Testing

```bash
# Test CLI streaming
golem chat
You: 你好
# Should see character-by-character output
```

### 8.2 WebChat Testing

```bash
# Test WebChat streaming
golem web --port 8080
# Open browser, send message
# Should see streaming output with cursor
```

### 8.3 Feishu Testing

```bash
# Test Feishu streaming
golem start
# Send message to Feishu bot
# Should see card updates
```

## 9. Implementation Order

1. **Phase 1: CLI Streaming**
   - Update `cmd/golem/chat.go`
   - Use `ChatStream()` instead of `Chat()`
   - Add typewriter effect

2. **Phase 2: WebChat Streaming**
   - Add `/api/stream` endpoint
   - Update Vue frontend
   - Add SSE support

3. **Phase 3: Feishu Streaming**
   - Implement StreamingManager
   - Add Card Kit support
   - Add throttle logic

## 10. Configuration

### 10.1 Streaming Config

```yaml
# ~/.golem/golem.yaml

streaming:
  enabled: true
  cli:
    delay_ms: 20          # Typewriter delay
  webchat:
    enabled: true
  feishu:
    enabled: true
    throttle_ms: 160      # Update throttle
```

## Appendix A: References

- [Server-Sent Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)
- [Feishu Card Kit](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/cardkit/overview)
- [OpenAI Streaming](https://platform.openai.com/docs/api-reference/chat/streaming)
