package openai

import (
	"strings"
	"testing"
)

func TestStreamParser_Parse(t *testing.T) {
	input := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\ndata: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\ndata: [DONE]\n"

	parser := NewStreamParser(strings.NewReader(input))

	var contents []string
	for chunk := range parser.Parse() {
		if chunk.Error != nil {
			t.Fatalf("unexpected error: %v", chunk.Error)
		}
		if chunk.Done {
			break
		}
		contents = append(contents, chunk.Content)
	}

	if len(contents) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(contents))
	}
	if contents[0] != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", contents[0])
	}
	if contents[1] != " world" {
		t.Errorf("expected ' world', got '%s'", contents[1])
	}
}

func TestStreamParser_EmptyInput(t *testing.T) {
	parser := NewStreamParser(strings.NewReader(""))
	for chunk := range parser.Parse() {
		if chunk.Error != nil {
			t.Fatalf("unexpected error: %v", chunk.Error)
		}
	}
}

func TestThinkingFilter_NoThinking(t *testing.T) {
	filter := NewThinkingFilter()
	result := filter.Filter("Hello, world!")
	if result != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got '%s'", result)
	}
}

func TestThinkingFilter_SingleChunk(t *testing.T) {
	filter := NewThinkingFilter()
	result := filter.Filter("<think>\nreasoning\n</think>\n\nActual response")
	if result != "\n\nActual response" {
		t.Errorf("expected '\\n\\nActual response', got '%s'", result)
	}
}

func TestThinkingFilter_SplitAcrossChunks(t *testing.T) {
	filter := NewThinkingFilter()

	// First chunk: start of thinking block
	result1 := filter.Filter("<think>\nreasoning")
	if result1 != "" {
		t.Errorf("expected empty, got '%s'", result1)
	}

	// Second chunk: end of thinking block and actual content
	result2 := filter.Filter("\n</think>\n\nActual response")
	if result2 != "\n\nActual response" {
		t.Errorf("expected '\\n\\nActual response', got '%s'", result2)
	}
}

func TestThinkingFilter_MultipleBlocks(t *testing.T) {
	filter := NewThinkingFilter()

	result1 := filter.Filter("Start ")
	if result1 != "Start " {
		t.Errorf("expected 'Start ', got '%s'", result1)
	}

	result2 := filter.Filter("<think>\nfirst\n</think>\n\nMiddle ")
	if result2 != "\n\nMiddle " {
		t.Errorf("expected '\\n\\nMiddle ', got '%s'", result2)
	}

	result3 := filter.Filter("<think>\nsecond\n</think>\n\nEnd")
	if result3 != "\n\nEnd" {
		t.Errorf("expected '\\n\\nEnd', got '%s'", result3)
	}
}

func TestThinkingFilter_PartialTag(t *testing.T) {
	filter := NewThinkingFilter()

	// Send partial "<think>" tag
	result1 := filter.Filter("Hello <th")
	if result1 != "Hello " {
		t.Errorf("expected 'Hello ', got '%s'", result1)
	}

	// Complete the tag
	result2 := filter.Filter("ink>reasoning</think>\n\nWorld")
	if result2 != "\n\nWorld" {
		t.Errorf("expected '\\n\\nWorld', got '%s'", result2)
	}
}
