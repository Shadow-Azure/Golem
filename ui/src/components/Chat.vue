<template>
  <div class="chat-container">
    <!-- Header -->
    <div class="chat-header">
      <h1>Golem AI Chat</h1>
      <button class="clear-btn" @click="clearChat" :disabled="isLoading">
        Clear
      </button>
    </div>

    <!-- Messages -->
    <div class="messages" ref="messagesContainer">
      <div v-if="messages.length === 0" class="empty-state">
        <p>Start a conversation with Golem AI.</p>
      </div>
      <div
        v-for="(msg, index) in messages"
        :key="index"
        :class="['message', msg.role]"
      >
        <div class="message-role">{{ msg.role === 'user' ? 'You' : 'Golem' }}</div>
        <div class="message-content">{{ msg.content }}</div>
      </div>
      <!-- Streaming indicator -->
      <div v-if="streamingContent" class="message assistant">
        <div class="message-role">Golem</div>
        <div class="message-content">{{ streamingContent }}<span class="cursor">|</span></div>
      </div>
    </div>

    <!-- Error display -->
    <div v-if="error" class="error-bar">
      {{ error }}
      <button class="error-dismiss" @click="error = ''">&times;</button>
    </div>

    <!-- Input -->
    <div class="input-area">
      <textarea
        v-model="input"
        @keydown.enter.exact="sendMessage"
        placeholder="Type a message... (Enter to send, Shift+Enter for newline)"
        :disabled="isLoading"
        rows="1"
        ref="inputField"
      ></textarea>
      <button
        class="send-btn"
        @click="sendMessage"
        :disabled="!input.trim() || isLoading"
      >
        Send
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, nextTick, onMounted, onUnmounted } from 'vue'

const messages = ref([])
const input = ref('')
const isLoading = ref(false)
const error = ref('')
const streamingContent = ref('')
const messagesContainer = ref(null)
const inputField = ref(null)

const SESSION_ID = 'webchat-' + Date.now()
const USER_ID = 'web-user'

let ws = null
let useWebSocket = false

// ---------- WebSocket connection ----------

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws`

  try {
    ws = new WebSocket(wsUrl)

    ws.onopen = () => {
      useWebSocket = true
      console.log('[ws] connected')
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        handleDaemonMessage(msg)
      } catch (e) {
        console.error('[ws] parse error:', e)
      }
    }

    ws.onclose = () => {
      useWebSocket = false
      console.log('[ws] disconnected')
    }

    ws.onerror = () => {
      useWebSocket = false
      console.warn('[ws] error, falling back to HTTP')
    }
  } catch {
    useWebSocket = false
  }
}

function handleDaemonMessage(msg) {
  switch (msg.type) {
    case 'chat_delta':
      streamingContent.value += msg.delta
      scrollToBottom()
      break
    case 'chat_done':
      messages.value.push({ role: 'assistant', content: msg.full_response })
      streamingContent.value = ''
      isLoading.value = false
      scrollToBottom()
      break
    case 'chat_error':
      error.value = msg.error || 'Unknown error'
      streamingContent.value = ''
      isLoading.value = false
      break
  }
}

// ---------- HTTP fallback ----------

async function sendMessage(event) {
  if (event && event.shiftKey) return // allow Shift+Enter for newline
  if (event) event.preventDefault()

  const text = input.value.trim()
  if (!text || isLoading.value) return

  // Add user message
  messages.value.push({ role: 'user', content: text })
  input.value = ''
  isLoading.value = true
  error.value = ''
  scrollToBottom()

  if (useWebSocket && ws && ws.readyState === WebSocket.OPEN) {
    sendViaWebSocket(text)
  } else {
    await sendViaHttp(text)
  }
}

function sendViaWebSocket(text) {
  streamingContent.value = ''
  const chatRequest = {
    type: 'chat_request',
    session_id: SESSION_ID,
    user_id: USER_ID,
    content: text,
    channel: 'webchat',
  }
  ws.send(JSON.stringify(chatRequest))
}

async function sendViaHttp(text) {
  try {
    const response = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: text }),
    })

    const data = await response.json()

    if (!response.ok) {
      throw new Error(data.error || `HTTP ${response.status}`)
    }

    messages.value.push({ role: 'assistant', content: data.content })
  } catch (e) {
    error.value = e.message || 'Failed to send message'
  } finally {
    isLoading.value = false
    scrollToBottom()
  }
}

// ---------- Clear chat ----------

async function clearChat() {
  if (isLoading.value) return

  try {
    await fetch('/api/clear', { method: 'POST' })
  } catch {
    // Ignore clear errors
  }

  messages.value = []
  streamingContent.value = ''
  error.value = ''
  scrollToBottom()
}

// ---------- Load history ----------

async function loadHistory() {
  try {
    const response = await fetch('/api/history')
    if (!response.ok) return

    const data = await response.json()
    if (data.messages && data.messages.length > 0) {
      messages.value = data.messages
      scrollToBottom()
    }
  } catch {
    // History not available, start fresh
  }
}

// ---------- Helpers ----------

function scrollToBottom() {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
}

// ---------- Lifecycle ----------

onMounted(() => {
  connectWebSocket()
  loadHistory()
  inputField.value?.focus()
})

onUnmounted(() => {
  if (ws) {
    ws.close()
    ws = null
  }
})
</script>

<style scoped>
.chat-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  max-width: 800px;
  margin: 0 auto;
  background: #fff;
  box-shadow: 0 0 20px rgba(0, 0, 0, 0.08);
}

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #e8e8e8;
  background: #fafafa;
}

.chat-header h1 {
  font-size: 18px;
  font-weight: 600;
  color: #1a1a1a;
}

.clear-btn {
  padding: 6px 14px;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  background: #fff;
  color: #666;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.clear-btn:hover:not(:disabled) {
  border-color: #ff4d4f;
  color: #ff4d4f;
}

.clear-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  color: #999;
  font-size: 15px;
}

.message {
  max-width: 80%;
  padding: 10px 14px;
  border-radius: 12px;
  line-height: 1.5;
  font-size: 14px;
  white-space: pre-wrap;
  word-break: break-word;
}

.message.user {
  align-self: flex-end;
  background: #1677ff;
  color: #fff;
  border-bottom-right-radius: 4px;
}

.message.assistant {
  align-self: flex-start;
  background: #f5f5f5;
  color: #1a1a1a;
  border-bottom-left-radius: 4px;
}

.message-role {
  font-size: 11px;
  font-weight: 600;
  margin-bottom: 4px;
  opacity: 0.7;
}

.message-content {
  font-size: 14px;
  line-height: 1.6;
}

.cursor {
  animation: blink 1s infinite;
  font-weight: 300;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

.error-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: #fff2f0;
  border: 1px solid #ffccc7;
  color: #cf1322;
  font-size: 13px;
}

.error-dismiss {
  background: none;
  border: none;
  color: #cf1322;
  font-size: 18px;
  cursor: pointer;
  padding: 0 4px;
}

.input-area {
  display: flex;
  gap: 10px;
  padding: 16px 20px;
  border-top: 1px solid #e8e8e8;
  background: #fafafa;
}

.input-area textarea {
  flex: 1;
  padding: 10px 14px;
  border: 1px solid #d9d9d9;
  border-radius: 8px;
  font-size: 14px;
  font-family: inherit;
  resize: none;
  outline: none;
  transition: border-color 0.2s;
  line-height: 1.5;
}

.input-area textarea:focus {
  border-color: #1677ff;
}

.input-area textarea:disabled {
  background: #f5f5f5;
  cursor: not-allowed;
}

.send-btn {
  padding: 10px 20px;
  border: none;
  border-radius: 8px;
  background: #1677ff;
  color: #fff;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
  align-self: flex-end;
}

.send-btn:hover:not(:disabled) {
  background: #4096ff;
}

.send-btn:disabled {
  background: #d9d9d9;
  cursor: not-allowed;
}
</style>
