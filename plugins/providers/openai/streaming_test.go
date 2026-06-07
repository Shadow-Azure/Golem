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
