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
	return &SSEWriter{
		w: w,
		f: w.(http.Flusher),
	}
}

// WriteContent writes a content chunk
func (s *SSEWriter) WriteContent(content string) error {
	data, _ := json.Marshal(map[string]string{"content": content})
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	s.f.Flush()
	return nil
}

// WriteError writes an error event
func (s *SSEWriter) WriteError(msg string) error {
	data, _ := json.Marshal(map[string]string{"error": msg})
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	s.f.Flush()
	return nil
}

// WriteDone writes a done event
func (s *SSEWriter) WriteDone() error {
	data, _ := json.Marshal(map[string]bool{"done": true})
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	s.f.Flush()
	return nil
}
