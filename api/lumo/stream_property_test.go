package lumo

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

// TestSSE_SerializeParseRoundTrip_Property verifies that for any valid
// GenerationResponseMessage, serializing with MarshalSSE then parsing
// the resulting data: line produces an equivalent message.
//
// Feature: lumo-api, Property 3: SSE serialize → parse round-trip
//
// **Validates: Requirements 3.1, 3.2**
func TestSSE_SerializeParseRoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		orig := genResponseMessage(t)

		data, err := MarshalSSE(orig)
		if err != nil {
			t.Fatalf("MarshalSSE: %v", err)
		}

		// The SSE line ends with \n\n. parseLine expects a single line
		// without trailing newlines, so trim them.
		line := string(bytes.TrimRight(data, "\n"))

		got, ok := parseLine(line)
		if !ok {
			t.Fatalf("parseLine returned false for MarshalSSE output: %q", line)
		}

		if !reflect.DeepEqual(orig, got) {
			t.Fatalf("round-trip mismatch:\norig: %+v\ngot:  %+v", orig, got)
		}
	})
}

// genNonTerminalResponseMessage generates a non-terminal message for use
// in multi-message stream tests.
func genNonTerminalResponseMessage(t *rapid.T) GenerationResponseMessage {
	nonTerminalTypes := []string{"queued", "ingesting", "token_data", "image_data"}
	typ := nonTerminalTypes[rapid.IntRange(0, len(nonTerminalTypes)-1).Draw(t, "msg_type")]

	msg := GenerationResponseMessage{Type: typ}
	switch typ {
	case "queued":
		if rapid.Bool().Draw(t, "has_target") {
			msg.Target = genTarget(t)
		}
	case "ingesting":
		msg.Target = genTarget(t)
	case "token_data":
		msg.Target = genTarget(t)
		msg.Count = rapid.IntRange(0, 10000).Draw(t, "count")
		msg.Content = rapid.StringMatching(`[a-zA-Z0-9 ]{0,32}`).Draw(t, "content")
		msg.Encrypted = rapid.Bool().Draw(t, "encrypted")
	case "image_data":
		if rapid.Bool().Draw(t, "has_image_id") {
			msg.ImageID = rapid.StringMatching(`[a-zA-Z0-9]{1,32}`).Draw(t, "image_id")
		}
		if rapid.Bool().Draw(t, "has_data") {
			msg.Data = rapid.StringMatching(`[a-zA-Z0-9+/=]{0,64}`).Draw(t, "data")
		}
		msg.IsFinal = rapid.Bool().Draw(t, "is_final")
		msg.Seed = rapid.IntRange(0, 999999).Draw(t, "seed")
		msg.Encrypted = rapid.Bool().Draw(t, "encrypted")
	}
	return msg
}

// buildSSEStream serializes a sequence of messages into a single SSE byte stream.
func buildSSEStream(msgs []GenerationResponseMessage) ([]byte, error) {
	var buf bytes.Buffer
	for _, msg := range msgs {
		data, err := MarshalSSE(msg)
		if err != nil {
			return nil, err
		}
		buf.Write(data)
	}
	return buf.Bytes(), nil
}

// processAll runs StreamProcessor.Process on the given reader and collects
// all messages. It ignores the terminal error since we're comparing message
// sequences.
func processAll(data []byte) []GenerationResponseMessage {
	var msgs []GenerationResponseMessage
	p := &StreamProcessor{}
	_ = p.Process(context.Background(), bytes.NewReader(data), func(msg GenerationResponseMessage) {
		msgs = append(msgs, msg)
	})
	return msgs
}

// chunkedReader is an io.Reader that delivers data in specified chunk sizes,
// simulating network read boundaries.
type chunkedReader struct {
	data   []byte
	chunks []int // sizes of each read
	idx    int   // current chunk index
	pos    int   // current position in data
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	size := len(r.data) - r.pos // default: rest of data
	if r.idx < len(r.chunks) {
		size = r.chunks[r.idx]
		r.idx++
	}
	if size > len(r.data)-r.pos {
		size = len(r.data) - r.pos
	}
	if size > len(p) {
		size = len(p)
	}
	n := copy(p[:size], r.data[r.pos:r.pos+size])
	r.pos += n
	return n, nil
}

// TestSSE_ChunkedParsing_Property verifies that for any sequence of valid
// SSE data: lines and any set of byte split points, feeding the stream to
// StreamProcessor in chunks split at those points produces the same
// sequence of messages as feeding the entire stream at once.
//
// Feature: lumo-api, Property 4: Chunked SSE parsing invariant
//
// **Validates: Requirements 2.1, 2.2, 2.7**
func TestSSE_ChunkedParsing_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 1-5 non-terminal messages followed by a done message.
		n := rapid.IntRange(1, 5).Draw(t, "num_msgs")
		msgs := make([]GenerationResponseMessage, 0, n+1)
		for i := 0; i < n; i++ {
			msgs = append(msgs, genNonTerminalResponseMessage(t))
		}
		msgs = append(msgs, GenerationResponseMessage{Type: "done"})

		stream, err := buildSSEStream(msgs)
		if err != nil {
			t.Fatalf("buildSSEStream: %v", err)
		}

		// Process the whole stream at once for the reference result.
		reference := processAll(stream)

		if len(stream) <= 1 {
			return
		}

		// Generate random chunk sizes (1 to len(stream)).
		numChunks := rapid.IntRange(2, min(20, len(stream))).Draw(t, "num_chunks")
		chunks := make([]int, numChunks)
		for i := range chunks {
			chunks[i] = rapid.IntRange(1, len(stream)).Draw(t, "chunk_size")
		}

		// Feed through chunked reader.
		cr := &chunkedReader{data: stream, chunks: chunks}
		var chunked []GenerationResponseMessage
		p := &StreamProcessor{}
		_ = p.Process(context.Background(), cr, func(msg GenerationResponseMessage) {
			chunked = append(chunked, msg)
		})

		if !reflect.DeepEqual(reference, chunked) {
			t.Fatalf("chunked parsing mismatch:\nreference: %+v\nchunked:   %+v\nchunks: %v", reference, chunked, chunks)
		}
	})
}

// TestSSE_GarbageResilience_Property verifies that for any sequence of
// valid SSE data: lines with arbitrary non-SSE lines injected between
// them, StreamProcessor produces the same messages as processing the
// stream without the injected lines.
//
// Feature: lumo-api, Property 5: Garbage line resilience
//
// **Validates: Requirements 2.3**
func TestSSE_GarbageResilience_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 1-5 non-terminal messages followed by a done message.
		n := rapid.IntRange(1, 5).Draw(t, "num_msgs")
		msgs := make([]GenerationResponseMessage, 0, n+1)
		for i := 0; i < n; i++ {
			msgs = append(msgs, genNonTerminalResponseMessage(t))
		}
		msgs = append(msgs, GenerationResponseMessage{Type: "done"})

		// Build clean stream and get reference messages.
		cleanStream, err := buildSSEStream(msgs)
		if err != nil {
			t.Fatalf("buildSSEStream: %v", err)
		}
		reference := processAll(cleanStream)

		// Build a dirty stream by injecting garbage lines between SSE lines.
		var dirty bytes.Buffer
		for _, msg := range msgs {
			// Inject 0-3 garbage lines before each message.
			numGarbage := rapid.IntRange(0, 3).Draw(t, "num_garbage")
			for j := 0; j < numGarbage; j++ {
				garbage := rapid.StringMatching(`[a-zA-Z0-9: ]{0,40}`).Draw(t, "garbage")
				dirty.WriteString(garbage)
				dirty.WriteByte('\n')
			}
			data, err := MarshalSSE(msg)
			if err != nil {
				t.Fatalf("MarshalSSE: %v", err)
			}
			dirty.Write(data)
		}

		dirtyMsgs := processAll(dirty.Bytes())

		if !reflect.DeepEqual(reference, dirtyMsgs) {
			t.Fatalf("garbage resilience mismatch:\nreference: %+v\ndirty:     %+v", reference, dirtyMsgs)
		}
	})
}
