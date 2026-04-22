package lumo

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStreamProcessor_TerminalMessages(t *testing.T) {
	tests := []struct {
		name    string
		msgType string
		wantErr error
	}{
		{"done", "done", nil},
		{"error", "error", ErrStreamClosed},
		{"rejected", "rejected", ErrRejected},
		{"harmful", "harmful", ErrHarmful},
		{"timeout", "timeout", ErrTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := `data: {"type":"` + tt.msgType + `"}` + "\n"
			r := strings.NewReader(line)

			var got []GenerationResponseMessage
			p := &StreamProcessor{}
			err := p.Process(context.Background(), r, func(msg GenerationResponseMessage) {
				got = append(got, msg)
			})

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Process() error = %v, want %v", err, tt.wantErr)
			}
			if len(got) != 1 {
				t.Fatalf("got %d messages, want 1", len(got))
			}
			if got[0].Type != tt.msgType {
				t.Errorf("message type = %q, want %q", got[0].Type, tt.msgType)
			}
		})
	}
}

func TestStreamProcessor_ContextCancellation(t *testing.T) {
	// Create a stream with many lines so the processor has work to do.
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString(`data: {"type":"token_data","content":"tok"}` + "\n")
	}
	sb.WriteString(`data: {"type":"done"}` + "\n")

	ctx, cancel := context.WithCancel(context.Background())

	var count int
	p := &StreamProcessor{}
	err := p.Process(ctx, strings.NewReader(sb.String()), func(_ GenerationResponseMessage) {
		count++
		if count >= 3 {
			cancel()
		}
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Process() error = %v, want context.Canceled", err)
	}
	// Should have processed at least 3 but not all 100.
	if count < 3 {
		t.Errorf("processed %d messages, want at least 3", count)
	}
	if count >= 100 {
		t.Errorf("processed all %d messages, expected early stop", count)
	}
}

func TestStreamProcessor_NonTerminalMessages(t *testing.T) {
	input := strings.Join([]string{
		`data: {"type":"queued"}`,
		`data: {"type":"ingesting","target":"message"}`,
		`data: {"type":"token_data","target":"message","content":"hello","count":1}`,
		`data: {"type":"done"}`,
		"",
	}, "\n")

	var got []GenerationResponseMessage
	p := &StreamProcessor{}
	err := p.Process(context.Background(), strings.NewReader(input), func(msg GenerationResponseMessage) {
		got = append(got, msg)
	})

	if err != nil {
		t.Fatalf("Process() error = %v, want nil", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d messages, want 4", len(got))
	}
	if got[0].Type != "queued" {
		t.Errorf("msg[0].Type = %q, want queued", got[0].Type)
	}
	if got[2].Content != "hello" {
		t.Errorf("msg[2].Content = %q, want hello", got[2].Content)
	}
}

func TestStreamProcessor_SkipsMalformedLines(t *testing.T) {
	input := strings.Join([]string{
		`data: {"type":"queued"}`,
		`data: not-json`,
		`this is garbage`,
		`data: {"type":"done"}`,
		"",
	}, "\n")

	var got []GenerationResponseMessage
	p := &StreamProcessor{}
	err := p.Process(context.Background(), strings.NewReader(input), func(msg GenerationResponseMessage) {
		got = append(got, msg)
	})

	if err != nil {
		t.Fatalf("Process() error = %v, want nil", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2 (queued + done)", len(got))
	}
}

func TestStreamProcessor_Finalize(t *testing.T) {
	p := &StreamProcessor{leftover: `data: {"type":"done"}`}
	msg, ok := p.Finalize()
	if !ok {
		t.Fatal("Finalize() returned false, want true")
	}
	if msg.Type != "done" {
		t.Errorf("Finalize() type = %q, want done", msg.Type)
	}

	// Second call should return nothing.
	_, ok = p.Finalize()
	if ok {
		t.Error("second Finalize() returned true, want false")
	}
}

func TestStreamProcessor_FinalizeEmpty(t *testing.T) {
	p := &StreamProcessor{}
	_, ok := p.Finalize()
	if ok {
		t.Error("Finalize() on empty processor returned true")
	}
}

func TestStreamProcessor_StreamClosedOnEOF(t *testing.T) {
	// A stream with no terminal message should return ErrStreamClosed.
	input := `data: {"type":"token_data","content":"partial"}` + "\n"

	var got []GenerationResponseMessage
	p := &StreamProcessor{}
	err := p.Process(context.Background(), strings.NewReader(input), func(msg GenerationResponseMessage) {
		got = append(got, msg)
	})

	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("Process() error = %v, want ErrStreamClosed", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d messages, want 1", len(got))
	}
}

func TestMarshalSSE(t *testing.T) {
	msg := GenerationResponseMessage{Type: "done"}
	data, err := MarshalSSE(msg)
	if err != nil {
		t.Fatalf("MarshalSSE() error = %v", err)
	}

	s := string(data)
	if !strings.HasPrefix(s, "data: ") {
		t.Errorf("MarshalSSE() missing 'data: ' prefix: %q", s)
	}
	if !strings.HasSuffix(s, "\n\n") {
		t.Errorf("MarshalSSE() missing trailing newlines: %q", s)
	}
}
