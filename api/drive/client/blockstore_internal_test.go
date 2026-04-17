package client

import (
	"bytes"
	"io"
	"testing"
)

func TestBlockReader_GetMultipartReader(t *testing.T) {
	data := []byte("test-block-data")
	br := &blockReader{r: bytes.NewReader(data)}

	reader := br.GetMultipartReader()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %q, want %q", got, data)
	}
}
