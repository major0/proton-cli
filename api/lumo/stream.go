package lumo

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// StreamProcessor parses SSE data: lines into typed messages.
// It buffers partial lines across read boundaries.
type StreamProcessor struct {
	leftover string
}

// Process reads from r, parses SSE data: lines, and calls fn for each
// parsed GenerationResponseMessage. Stops on context cancellation, done
// message, or terminal error message. Returns nil on done, the
// corresponding sentinel error on terminal messages, or a wrapped error
// on parse/read failures.
func (p *StreamProcessor) Process(ctx context.Context, r io.Reader, fn func(GenerationResponseMessage)) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		line := scanner.Text()

		// Prepend any leftover from a previous chunk.
		if p.leftover != "" {
			line = p.leftover + line
			p.leftover = ""
		}

		msg, ok := parseLine(line)
		if !ok {
			continue
		}

		fn(msg)

		if msg.IsTerminal() {
			return terminalError(msg.Type)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("lumo: read stream: %w", err)
	}

	return ErrStreamClosed
}

// Finalize flushes any buffered partial line. Returns at most one message.
func (p *StreamProcessor) Finalize() (GenerationResponseMessage, bool) {
	if p.leftover == "" {
		return GenerationResponseMessage{}, false
	}
	line := p.leftover
	p.leftover = ""
	return parseLine(line)
}

// MarshalSSE serializes a message to an SSE data: line.
// The output format is "data: <json>\n\n".
func MarshalSSE(msg GenerationResponseMessage) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("lumo: marshal SSE: %w", err)
	}

	var b strings.Builder
	b.Grow(len("data: ") + len(data) + 2)
	b.WriteString("data: ")
	b.Write(data)
	b.WriteString("\n\n")

	return []byte(b.String()), nil
}

// parseLine attempts to parse a single SSE line into a message.
// Returns false if the line is not a valid data: line or fails to parse.
func parseLine(line string) (GenerationResponseMessage, bool) {
	if !strings.HasPrefix(line, "data: ") && !strings.HasPrefix(line, "data:") {
		return GenerationResponseMessage{}, false
	}

	jsonStr := strings.TrimPrefix(line, "data:")
	jsonStr = strings.TrimPrefix(jsonStr, " ")

	var msg GenerationResponseMessage
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		slog.Debug("lumo: skipping malformed SSE data line", "error", err)
		return GenerationResponseMessage{}, false
	}

	if msg.Type == "" {
		slog.Debug("lumo: skipping SSE data line with empty type")
		return GenerationResponseMessage{}, false
	}

	return msg, true
}

// terminalError maps terminal message types to their sentinel errors.
// Returns nil for "done" (successful completion) and for non-terminal types.
func terminalError(typ string) error {
	switch typ {
	case "done":
		return nil
	case "error":
		return ErrStreamClosed
	case "rejected":
		return ErrRejected
	case "harmful":
		return ErrHarmful
	case "timeout":
		return ErrTimeout
	default:
		return nil
	}
}
