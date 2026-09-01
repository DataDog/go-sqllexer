// Package corpus reads and writes the newline-delimited request corpora consumed
// by the harness tools.
package corpus

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/DataDog/go-sqllexer/harness/internal/protocol"
)

// maxLine bounds a single corpus entry. The largest benchmark query in the repo is
// ~109KB, so 8MB leaves ample headroom for generated pathological inputs.
const maxLine = 8 << 20

// Read loads every request from a JSONL file.
func Read(path string) ([]protocol.Request, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadFrom(f)
}

// ReadFrom loads every request from a JSONL stream.
func ReadFrom(r io.Reader) ([]protocol.Request, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), maxLine)

	var requests []protocol.Request
	for line := 1; scanner.Scan(); line++ {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var req protocol.Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		requests = append(requests, req)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return requests, nil
}

// Writer streams requests to a JSONL file.
type Writer struct {
	f   *os.File
	bw  *bufio.Writer
	enc *json.Encoder
	n   int
}

// NewWriter creates the file at path and returns a streaming writer.
func NewWriter(path string) (*Writer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	bw := bufio.NewWriterSize(f, 1<<20)
	return &Writer{f: f, bw: bw, enc: json.NewEncoder(bw)}, nil
}

// Write appends one request.
func (w *Writer) Write(req protocol.Request) error {
	w.n++
	return w.enc.Encode(req)
}

// Count returns how many requests have been written.
func (w *Writer) Count() int { return w.n }

// Close flushes and closes the underlying file.
func (w *Writer) Close() error {
	if err := w.bw.Flush(); err != nil {
		w.f.Close()
		return err
	}
	return w.f.Close()
}
