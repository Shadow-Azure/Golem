<template>
  <div class="chat-app">
    <!-- Header -->
    <header class="header">
      <div class="header-content">
        <div class="logo">
          <div class="logo-icon">G</div>
          <div class="logo-text">
            <h1>Golem</h1>
            <span class="subtitle">AI Assistant</span>
          </div>
        </div>
        <div class="header-actions">
          <button class="btn-icon" @click="clearHistory" title="清空对话">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M3 6h18M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/>
            </svg>
          </button>
        </div>
      </div>
    </header>

    <!-- Messages -->
    <main class="messages" ref="messagesRef">
      <!-- Empty state -->
      <div v-if="messages.length === 0 && !streamingContent" class="empty-state">
        <div class="empty-card">
          <div class="empty-icon">✦</div>
          <h2>开始对话</h2>
          <p>输入消息开始与 AI 助手交流</p>
        </div>
      </div>

      <!-- Messages list -->
      <div
        v-for="(msg, index) in messages"
        :key="index"
        :class="['message', msg.role]"
        :style="{ animationDelay: `${index * 0.05}s` }"
      >
        <div v-if="msg.role === 'assistant'" class="avatar-row">
          <div class="avatar assistant-avatar">
            <span>G</span>
          </div>
          <div class="bubble assistant-bubble">
            <div class="bubble-header">
              <span class="bubble-name">Golem</span>
            </div>
            <div class="bubble-content" v-html="formatMessage(msg.content)"></div>
          </div>
        </div>

        <div v-else class="avatar-row user-row">
          <div class="bubble user-bubble">
            <div class="bubble-content">{{ msg.content }}</div>
          </div>
        </div>
      </div>

      <!-- Streaming message -->
      <div v-if="streamingContent" class="message assistant">
        <div class="avatar-row">
          <div class="avatar assistant-avatar">
            <span>G</span>
          </div>
          <div class="bubble assistant-bubble streaming">
            <div class="bubble-header">
              <span class="bubble-name">Golem</span>
              <span class="streaming-badge">输入中...</span>
            </div>
            <div class="bubble-content">
              {{ streamingContent }}<span class="cursor">│</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Loading state -->
      <div v-if="loading && !streamingContent" class="message assistant">
        <div class="avatar-row">
          <div class="avatar assistant-avatar">
            <span>G</span>
          </div>
          <div class="bubble assistant-bubble loading-bubble">
            <div class="bubble-header">
              <span class="bubble-name">Golem</span>
            </div>
            <div class="loading-dots">
              <span></span>
              <span></span>
              <span></span>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Input Area -->
    <footer class="input-area">
      <div class="input-container">
        <div class="input-card">
          <div class="input-wrapper">
            <textarea
              ref="inputRef"
              v-model="input"
              @keydown.enter.exact.prevent="sendMessageStream"
              placeholder="输入消息..."
              :disabled="isStreaming"
              rows="1"
              @input="autoResize"
            ></textarea>
            <button
              class="send-btn"
              @click="sendMessageStream"
              :disabled="isStreaming || !input.trim()"
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z"/>
              </svg>
            </button>
          </div>
        </div>
        <div class="input-hint">
          <span>Enter 发送 · Shift + Enter 换行</span>
        </div>
      </div>
    </footer>
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
const inputRef = ref(null)

const scrollToBottom = () => {
  if (messagesRef.value) {
    messagesRef.value.scrollTo({
      top: messagesRef.value.scrollHeight,
      behavior: 'smooth'
    })
  }
}

const autoResize = (e) => {
  const textarea = e.target
  textarea.style.height = 'auto'
  textarea.style.height = Math.min(textarea.scrollHeight, 180) + 'px'
}

const formatMessage = (content) => {
  let filtered = content.replace(/<think>[\s\S]*?<\/think>/g, '')
  return filtered
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    .replace(/`(.*?)`/g, '<code>$1</code>')
    .replace(/\n/g, '<br>')
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

  if (inputRef.value) {
    inputRef.value.style.height = 'auto'
  }

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
  if (inputRef.value) {
    inputRef.value.focus()
  }
})
</script>

<style scoped>
/* ===== Design Tokens ===== */
:root {
  --bg-page: #f0f2f5;
  --bg-header: #ffffff;
  --bg-input: #ffffff;

  --surface: #ffffff;

  --border: #d1d5db;
  --border-strong: #9ca3af;
  --border-accent: #5b6cf7;

  --text-primary: #111827;
  --text-secondary: #4b5563;
  --text-tertiary: #9ca3af;

  --accent: #5b6cf7;
  --accent-hover: #4a5be5;
  --accent-light: #eef1fe;
  --accent-glow: rgba(91, 108, 247, 0.25);

  --user-bg: #5b6cf7;
  --user-text: #ffffff;
  --user-border: #4a5be5;
  --user-shadow: 0 2px 12px rgba(91, 108, 247, 0.4);

  --assistant-bg: #ffffff;
  --assistant-text: #111827;
  --assistant-border: #d1d5db;
  --assistant-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);

  --radius-sm: 8px;
  --radius-md: 12px;
  --radius-lg: 16px;
  --radius-xl: 20px;
  --radius-2xl: 24px;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

/* ===== App Container ===== */
.chat-app {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-page);
  color: var(--text-primary);
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', Roboto, sans-serif;
  overflow: hidden;
}

/* ===== Header ===== */
.header {
  padding: 0 32px;
  height: 64px;
  display: flex;
  align-items: center;
  background: var(--bg-header);
  border-bottom: 1px solid var(--border);
  box-shadow: var(--shadow-xs);
  z-index: 10;
}

.header-content {
  width: 100%;
  max-width: 800px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.logo {
  display: flex;
  align-items: center;
  gap: 14px;
}

.logo-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--accent), #8b5cf6);
  border-radius: 12px;
  font-weight: 700;
  font-size: 18px;
  color: white;
  box-shadow: 0 4px 12px rgba(91, 108, 247, 0.3);
}

.logo-text h1 {
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--text-primary);
}

.logo-text .subtitle {
  font-size: 11px;
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-weight: 500;
}

.btn-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-icon:hover {
  background: var(--bg-page);
  color: var(--text-primary);
  border-color: var(--border-strong);
  transform: translateY(-1px);
  box-shadow: var(--shadow-sm);
}

/* ===== Messages ===== */
.messages {
  flex: 1;
  overflow-y: auto;
  padding: 32px;
  background: var(--bg-page);
}

.messages::-webkit-scrollbar {
  width: 6px;
}

.messages::-webkit-scrollbar-track {
  background: transparent;
}

.messages::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.15);
  border-radius: 3px;
}

.messages::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 0, 0, 0.25);
}

/* ===== Empty State ===== */
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  animation: fadeIn 0.5s ease;
}

.empty-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  padding: 48px 56px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-2xl);
  box-shadow: var(--shadow-lg);
}

.empty-icon {
  font-size: 48px;
  color: var(--accent);
  opacity: 0.8;
}

.empty-card h2 {
  font-size: 24px;
  font-weight: 600;
  color: var(--text-primary);
}

.empty-card p {
  font-size: 14px;
  color: var(--text-secondary);
}

/* ===== Messages ===== */
.message {
  margin-bottom: 24px;
  animation: slideUp 0.3s ease forwards;
  opacity: 0;
}

@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.avatar-row {
  display: flex;
  gap: 14px;
  max-width: 800px;
  margin: 0 auto;
}

.avatar-row.user-row {
  justify-content: flex-end;
}

/* ===== Avatar ===== */
.avatar {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 14px;
  flex-shrink: 0;
  box-shadow: var(--shadow-sm);
}

.assistant-avatar {
  background: linear-gradient(135deg, var(--accent), #8b5cf6);
  color: white;
}

/* ===== Bubbles ===== */
.bubble {
  max-width: 65%;
  border-radius: var(--radius-lg);
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}

/* User bubble - inline to fit content */
.user-bubble {
  display: inline-block;
  background: var(--user-bg);
  color: var(--user-text);
  padding: 12px 18px;
  border-bottom-right-radius: var(--radius-sm);
  border: 1.5px solid var(--user-border);
  box-shadow: var(--user-shadow);
  text-align: left;
}

/* Assistant bubble - block to fill width */
.assistant-bubble {
  display: block;
  background: var(--assistant-bg);
  border: 1.5px solid var(--assistant-border);
  padding: 16px 20px;
  box-shadow: var(--assistant-shadow);
  color: var(--assistant-text);
}

.bubble-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.bubble-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.streaming-badge {
  font-size: 11px;
  font-weight: 500;
  color: var(--accent);
  background: var(--accent-light);
  padding: 2px 10px;
  border-radius: 12px;
}

.bubble-content {
  letter-spacing: 0.01em;
  line-height: 1.7;
}

/* ===== Streaming ===== */
.streaming {
  border-color: var(--border-accent) !important;
  box-shadow: 0 0 0 3px var(--accent-glow), var(--shadow-md) !important;
}

.cursor {
  color: var(--accent);
  font-weight: 300;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

/* ===== Loading ===== */
.loading-bubble {
  padding: 20px 22px;
}

.loading-dots {
  display: flex;
  gap: 6px;
  padding: 4px 0;
}

.loading-dots span {
  width: 8px;
  height: 8px;
  background: var(--text-tertiary);
  border-radius: 50%;
  animation: dotPulse 1.4s ease-in-out infinite;
}

.loading-dots span:nth-child(2) { animation-delay: 0.2s; }
.loading-dots span:nth-child(3) { animation-delay: 0.4s; }

@keyframes dotPulse {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.4; }
  40% { transform: scale(1); opacity: 1; }
}

/* ===== Input Area ===== */
.input-area {
  padding: 20px 32px 28px;
  background: var(--bg-header);
  border-top: 1px solid var(--border);
  box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.04);
}

.input-container {
  max-width: 800px;
  margin: 0 auto;
}

.input-card {
  background: var(--bg-page);
  border: 2px solid var(--border);
  border-radius: var(--radius-xl);
  padding: 8px;
  transition: all 0.2s ease;
  box-shadow: var(--shadow-sm);
}

.input-card:focus-within {
  border-color: var(--accent);
  box-shadow: 0 0 0 4px var(--accent-glow), var(--shadow-md);
}

.input-wrapper {
  display: flex;
  gap: 10px;
  align-items: flex-end;
}

.input-wrapper textarea {
  flex: 1;
  padding: 12px 14px;
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 14.5px;
  font-family: inherit;
  line-height: 1.6;
  resize: none;
  outline: none;
  max-height: 160px;
}

.input-wrapper textarea::placeholder {
  color: var(--text-tertiary);
}

.input-wrapper textarea:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.send-btn {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--accent);
  border: none;
  border-radius: var(--radius-md);
  color: white;
  cursor: pointer;
  transition: all 0.2s ease;
  flex-shrink: 0;
}

.send-btn:hover:not(:disabled) {
  background: var(--accent-hover);
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(91, 108, 247, 0.35);
}

.send-btn:active:not(:disabled) {
  transform: translateY(0);
}

.send-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.input-hint {
  margin-top: 10px;
  text-align: center;
}

.input-hint span {
  font-size: 12px;
  color: var(--text-tertiary);
}

/* ===== Message Content Formatting ===== */
.bubble-content :deep(strong) {
  font-weight: 600;
}

.bubble-content :deep(em) {
  font-style: italic;
  color: var(--text-secondary);
}

.bubble-content :deep(code) {
  background: var(--bg-page);
  padding: 3px 8px;
  border-radius: 6px;
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  border: 1px solid var(--border);
}

.user-bubble .bubble-content :deep(code) {
  background: rgba(255, 255, 255, 0.2);
  border-color: rgba(255, 255, 255, 0.3);
}

/* ===== Animations ===== */
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

/* ===== Responsive ===== */
@media (max-width: 768px) {
  .header {
    padding: 0 20px;
  }

  .messages {
    padding: 20px;
  }

  .input-area {
    padding: 16px 20px 24px;
  }

  .bubble {
    max-width: 85%;
  }
}
</style>
