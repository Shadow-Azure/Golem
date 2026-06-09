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
