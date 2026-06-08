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

<style scoped>
.chat-container {
  max-width: 800px;
  margin: 0 auto;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: white;
  box-shadow: 0 0 20px rgba(0,0,0,0.1);
}

.header {
  padding: 20px;
  background: #007bff;
  color: white;
  text-align: center;
}

.header h1 {
  font-size: 1.5rem;
  font-weight: 600;
}

.messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background: #f5f5f5;
}

.message {
  display: flex;
  margin-bottom: 16px;
  gap: 12px;
}

.message.user {
  flex-direction: row-reverse;
}

.avatar {
  font-size: 24px;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: white;
  border-radius: 50%;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  flex-shrink: 0;
}

.content {
  max-width: 70%;
  padding: 12px 16px;
  border-radius: 12px;
  line-height: 1.5;
  word-break: break-word;
}

.message.user .content {
  background: #007bff;
  color: white;
  border-bottom-right-radius: 4px;
}

.message.assistant .content {
  background: white;
  color: #333;
  border-bottom-left-radius: 4px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

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

.input-area {
  display: flex;
  gap: 12px;
  padding: 20px;
  background: white;
  border-top: 1px solid #eee;
}

.input-area input {
  flex: 1;
  padding: 12px 16px;
  border: 2px solid #ddd;
  border-radius: 8px;
  font-size: 16px;
  outline: none;
  transition: border-color 0.2s;
}

.input-area input:focus {
  border-color: #007bff;
}

.input-area button {
  padding: 12px 24px;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 16px;
  cursor: pointer;
  transition: background 0.2s;
}

.input-area button:hover:not(:disabled) {
  background: #0056b3;
}

.input-area button:disabled {
  background: #ccc;
  cursor: not-allowed;
}

.clear-btn {
  background: #6c757d !important;
}

.clear-btn:hover:not(:disabled) {
  background: #545b62 !important;
}
</style>
