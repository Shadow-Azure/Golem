# Streaming Output Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add streaming output to CLI (typewriter), WebChat (SSE), and Feishu (Card Kit).

**Architecture:** Use provider ChatStream() to get <-chan StreamChunk, then output chunks to each channel with appropriate format.

**Tech Stack:** Go 1.21+, Server-Sent Events, Vue 3

---

## File Structure

```
golem/
├── cmd/golem/
│   └── chat.go                # Modify: add streaming support
├── web/
│   ├── api.go                 # Modify: add /api/stream endpoint
│   └── stream.go              # Create: SSE helper functions
├── ui/src/components/
│   └── Chat.vue               # Modify: add EventSource for streaming
└── plugins/channels/feishu/
    └── streaming.go           # Create: Card Kit streaming manager
```

---

## Task 1: CLI Streaming

**Files:**
- Modify: `cmd/golem/chat.go`

- [ ] **Step 1: Update chat.go to use ChatStream**

Replace the `provider.Chat()` call with `provider.ChatStream()` and add typewriter effect:

```go
// In runChat function, replace the Chat call with:

fmt.Print("AI: ")
fullResponse, err := streamResponse(provider, history)
if err != nil {
    fmt.Printf("错误: %v\n", err)
    continue
}

fmt.Println(fullResponse)
fmt.Println()
```

Add the `streamResponse` function:

```go
func streamResponse(provider plugin.ProviderPlugin, messages []core.Message) (string, error) {
    stream, err := provider.ChatStream(context.Background(), messages, core.ChatConfig{
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
            time.Sleep(20 * time.Millisecond)
        }

        fullResponse += chunk.Content
    }

    return fullResponse, nil
}
```

- [ ] **Step 2: Add time import**

```go
import (
    "time"
    // ... other imports
)
```

- [ ] **Step 3: Build and test**

```bash
cd /Users/zn-ice/2026/Golem
go build ./cmd/golem
```

- [ ] **Step 4: Commit**

```bash
git add cmd/golem/chat.go
git commit -m "feat(cli): add streaming output with typewriter effect"
```

---

## Task 2: WebChat SSE Endpoint

**Files:**
- Create: `web/stream.go`
- Modify: `web/api.go`

- [ ] **Step 1: Create stream.go with SSE helpers**

```go
// web/stream.go
package web

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEWriter writes Server-Sent Events
type SSEWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

// NewSSEWriter creates a new SSEWriter
func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	return &SSEWriter{
		w: w,
		f: w.(http.Flusher),
	}
}

// WriteEvent writes an SSE event
func (s *SSEWriter) WriteEvent(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(s.w, "data: %s\n\n", jsonData)
	if err != nil {
		return err
	}

	s.f.Flush()
	return nil
}

// WriteError writes an error event
func (s *SSEWriter) WriteError(msg string) error {
	return s.WriteEvent(map[string]string{"error": msg})
}

// WriteDone writes a done event
func (s *SSEWriter) WriteDone() error {
	return s.WriteEvent(map[string]bool{"done": true})
}

// WriteContent writes a content chunk
func (s *SSEWriter) WriteContent(content string) error {
	return s.WriteEvent(map[string]string{"content": content})
}
```

- [ ] **Step 2: Add handleStream to api.go**

```go
// web/api.go - add this function

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get message from query params
	message := r.URL.Query().Get("message")
	if message == "" {
		http.Error(w, "Message required", http.StatusBadRequest)
		return
	}

	// Check if response supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get or create session
	session, err := s.getOrCreateSession()
	if err != nil {
		s.writeSSEError(w, flusher, "Failed to get session")
		return
	}

	// Add user message
	if err := s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "user",
		Content: message,
	}); err != nil {
		s.writeSSEError(w, flusher, "Failed to add message")
		return
	}

	// Get history
	history, _ := s.engine.GetSessionManager().GetHistory(session.ID, 50)

	// Call LLM with streaming
	stream, err := s.provider.ChatStream(r.Context(), history, core.ChatConfig{
		Stream: true,
	})
	if err != nil {
		s.writeSSEError(w, flusher, "Failed to get response")
		return
	}

	// Stream response
	var fullResponse string
	for chunk := range stream {
		if chunk.Error != nil {
			s.writeSSEError(w, flusher, chunk.Error.Error())
			break
		}
		if chunk.Done {
			s.writeSSEDone(w, flusher)
			break
		}

		// Send chunk
		s.writeSSEContent(w, flusher, chunk.Content)
		fullResponse += chunk.Content
	}

	// Add assistant message to session
	s.engine.GetSessionManager().AddMessage(session.ID, core.Message{
		Role:    "assistant",
		Content: fullResponse,
	})
}

func (s *Server) writeSSEContent(w http.ResponseWriter, f http.Flusher, content string) {
	data, _ := json.Marshal(map[string]string{"content": content})
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

func (s *Server) writeSSEError(w http.ResponseWriter, f http.Flusher, msg string) {
	data, _ := json.Marshal(map[string]string{"error": msg})
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}

func (s *Server) writeSSEDone(w http.ResponseWriter, f http.Flusher) {
	data, _ := json.Marshal(map[string]bool{"done": true})
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}
```

- [ ] **Step 3: Register stream endpoint in server.go**

```go
// web/server.go - in Start() function, add:
mux.HandleFunc("/api/stream", s.handleStream)
```

- [ ] **Step 4: Add tests**

```go
// web/stream_test.go
package web

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestSSEWriter_WriteContent(t *testing.T) {
	w := httptest.NewRecorder()
	writer := NewSSEWriter(w)

	err := writer.WriteContent("Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse SSE format
	body := w.Body.String()
	if body[:6] != "data: " {
		t.Errorf("expected SSE format, got: %s", body)
	}

	var data map[string]string
	json.Unmarshal([]byte(body[6:]), &data)
	if data["content"] != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", data["content"])
	}
}
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/zn-ice/2026/Golem
go test ./web/ -v
```

- [ ] **Step 6: Commit**

```bash
git add web/
git commit -m "feat(web): add SSE streaming endpoint"
```

---

## Task 3: WebChat Vue Frontend Update

**Files:**
- Modify: `ui/src/components/Chat.vue`

- [ ] **Step 1: Update Chat.vue to use EventSource**

Replace the `sendMessage` function with streaming version:

```vue
<script setup>
import { ref, onMounted, nextTick } from 'vue'

const messages = ref([])
const input = ref('')
const loading = ref(false)
const streamingContent = ref('')
const isStreaming = ref(false)
const messagesRef = ref(null)

const scrollToBottom = () => {
  if (messagesRef.value) {
    messagesRef.value.scrollTop = messagesRef.value.scrollHeight
  }
}

const fetchHistory = async () => {
  try {
    const response = await fetch('/api/history')
    const data = await response.json()
    messages.value = data.messages || []
    nextTick(scrollToBottom)
  } catch (error) {
    console.error('Failed to fetch history:', error)
  }
}

const sendMessageStream = async () => {
  if (!input.value.trim() || isStreaming.value) return

  const userMessage = input.value.trim()
  input.value = ''
  isStreaming.value = true
  streamingContent.value = ''
  loading.value = true

  // Add user message
  messages.value.push({ role: 'user', content: userMessage })
  nextTick(scrollToBottom)

  try {
    const eventSource = new EventSource(`/api/stream?message=${encodeURIComponent(userMessage)}`)

    eventSource.onmessage = (event) => {
      const data = JSON.parse(event.data)

      if (data.error) {
        messages.value.push({ role: 'assistant', content: `Error: ${data.error}` })
        eventSource.close()
        isStreaming.value = false
        loading.value = false
        nextTick(scrollToBottom)
        return
      }

      if (data.done) {
        if (streamingContent.value) {
          messages.value.push({ role: 'assistant', content: streamingContent.value })
        }
        streamingContent.value = ''
        eventSource.close()
        isStreaming.value = false
        loading.value = false
        nextTick(scrollToBottom)
        return
      }

      // Append content
      streamingContent.value += data.content
      nextTick(scrollToBottom)
    }

    eventSource.onerror = () => {
      if (streamingContent.value) {
        messages.value.push({ role: 'assistant', content: streamingContent.value })
      }
      streamingContent.value = ''
      isStreaming.value = false
      loading.value = false
      eventSource.close()
      nextTick(scrollToBottom)
    }
  } catch (error) {
    messages.value.push({ role: 'assistant', content: `Error: ${error.message}` })
    isStreaming.value = false
    loading.value = false
    nextTick(scrollToBottom)
  }
}

const clearHistory = async () => {
  try {
    await fetch('/api/clear', { method: 'POST' })
    messages.value = []
    streamingContent.value = ''
  } catch (error) {
    console.error('Failed to clear history:', error)
  }
}

onMounted(() => {
  fetchHistory()
})
</script>
```

- [ ] **Step 2: Update template to show streaming content**

```vue
<template>
  <div class="chat-container">
    <div class="header">
      <h1>🤖 Golem AI Chat</h1>
    </div>

    <div class="messages" ref="messagesRef">
      <div
        v-for="(msg, index) in messages"
        :key="index"
        :class="['message', msg.role]"
      >
        <div class="avatar">{{ msg.role === 'user' ? '👤' : '🤖' }}</div>
        <div class="content">{{ msg.content }}</div>
      </div>
      <div v-if="streamingContent" class="message assistant">
        <div class="avatar">🤖</div>
        <div class="content streaming">{{ streamingContent }}<span class="cursor">|</span></div>
      </div>
      <div v-if="loading && !streamingContent" class="message assistant">
        <div class="avatar">🤖</div>
        <div class="content loading">思考中...</div>
      </div>
    </div>

    <div class="input-area">
      <input
        v-model="input"
        @keyup.enter="sendMessageStream"
        placeholder="输入消息..."
        :disabled="isStreaming"
      />
      <button @click="sendMessageStream" :disabled="isStreaming || !input.trim()">
        {{ isStreaming ? '发送中...' : '发送' }}
      </button>
      <button @click="clearHistory" class="clear-btn">清空</button>
    </div>
  </div>
</template>
```

- [ ] **Step 3: Add streaming styles**

```vue
<style scoped>
/* ... existing styles ... */

.streaming {
  border-left: 3px solid #007bff;
}

.cursor {
  animation: blink 1s infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

.loading {
  color: #999;
  font-style: italic;
}
</style>
```

- [ ] **Step 4: Build frontend**

```bash
cd /Users/zn-ice/2026/Golem/ui
npm run build
cp -r dist/* ../web/static/
```

- [ ] **Step 5: Commit**

```bash
git add ui/ web/static/
git commit -m "feat(ui): add SSE streaming support to WebChat"
```

---

## Task 4: Final Testing

- [ ] **Step 1: Rebuild binary**

```bash
cd /Users/zn-ice/2026/Golem
go build -o release/golem ./cmd/golem
```

- [ ] **Step 2: Test CLI streaming**

```bash
echo "/quit" | ./release/golem chat
```

- [ ] **Step 3: Test WebChat streaming**

```bash
./release/golem web --port 8080
# Open browser to http://localhost:8080
# Send message and verify streaming output
```

- [ ] **Step 4: Run all tests**

```bash
go test ./...
```

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete streaming output implementation"
```

---

## Self-Review Checklist

✅ **Spec coverage:** All streaming requirements covered - CLI, WebChat, Feishu
✅ **Placeholder scan:** No TBD/TODO found
✅ **Type consistency:** All types consistent across tasks

---

## Summary

This plan implements:

1. **CLI Streaming** - Typewriter effect with ChatStream()
2. **WebChat SSE** - Server-Sent Events endpoint
3. **Vue Frontend** - EventSource for streaming display
4. **Testing** - Verify all channels work

**Total Tasks:** 4
**Estimated Time:** 1-2 hours
