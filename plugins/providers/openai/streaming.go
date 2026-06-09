package openai

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/Shadow-Azure/Golem/internal/core"
)

// StreamParser parses SSE streaming responses from the OpenAI API.
type StreamParser struct {
	reader *bufio.Reader
}

// ThinkingFilter filters out <think>...</think> blocks from streaming content.
type ThinkingFilter struct {
	inThinking bool
	buffer     string
}

// NewThinkingFilter creates a new ThinkingFilter.
func NewThinkingFilter() *ThinkingFilter {
	return &ThinkingFilter{}
}

// Filter processes a chunk of content and returns the filtered output.
// It buffers content to detect thinking block boundaries across chunks.
func (f *ThinkingFilter) Filter(chunk string) string {
	f.buffer += chunk
	var output strings.Builder

	for {
		if f.inThinking {
			// Look for closing tag
			idx := strings.Index(f.buffer, "</think>")
			if idx == -1 {
				// Still in thinking block - check for partial closing tag at end
				for i := len(f.buffer); i > 0; i-- {
					suffix := "</think>"
					if strings.HasPrefix(suffix, f.buffer[i-1:]) {
						// Buffer ends with partial closing tag, keep it in buffer
						f.buffer = f.buffer[i-1:]
						return output.String()
					}
				}
				// No partial tag, consume all buffered content (still in thinking)
				f.buffer = ""
				return output.String()
			}
			// Found closing tag, skip past it
			f.buffer = f.buffer[idx+len("</think>"):]
			f.inThinking = false
		} else {
			// Look for opening tag
			idx := strings.Index(f.buffer, "<think>")
			if idx == -1 {
				// No thinking block, output everything except trailing partial tag
				// Check if buffer ends with partial "<think>" tag
				for i := len(f.buffer); i > 0; i-- {
					prefix := "<think>"
					if strings.HasPrefix(prefix, f.buffer[i-1:]) {
						// Buffer ends with partial tag, keep it in buffer
						output.WriteString(f.buffer[:i-1])
						f.buffer = f.buffer[i-1:]
						return output.String()
					}
				}
				// No partial tag, output everything
				output.WriteString(f.buffer)
				f.buffer = ""
				return output.String()
			}
			// Found opening tag, output content before it
			output.WriteString(f.buffer[:idx])
			f.buffer = f.buffer[idx+len("<think>"):]
			f.inThinking = true
		}
	}
}

// NewStreamParser creates a new StreamParser from an io.Reader.
func NewStreamParser(reader io.Reader) *StreamParser {
	return &StreamParser{reader: bufio.NewReader(reader)}
}

// Parse returns a channel of StreamChunks parsed from the SSE stream.
func (p *StreamParser) Parse() <-chan core.StreamChunk {
	chunks := make(chan core.StreamChunk, 100)

	go func() {
		defer close(chunks)

		for {
			line, err := p.reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					chunks <- core.StreamChunk{Error: err}
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				chunks <- core.StreamChunk{Done: true}
				return
			}

			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				chunks <- core.StreamChunk{Content: chunk.Choices[0].Delta.Content}
			}
		}
	}()

	return chunks
}
