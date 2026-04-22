package lumo

import "errors"

var (
	// ErrStreamClosed indicates the SSE stream was closed unexpectedly.
	ErrStreamClosed = errors.New("lumo: stream closed")
	// ErrRejected indicates the request was rejected by the backend.
	ErrRejected = errors.New("lumo: request rejected")
	// ErrHarmful indicates harmful content was detected.
	ErrHarmful = errors.New("lumo: harmful content detected")
	// ErrTimeout indicates the generation timed out.
	ErrTimeout = errors.New("lumo: generation timeout")
	// ErrDecryptionFailed indicates a decryption operation failed.
	ErrDecryptionFailed = errors.New("lumo: decryption failed")
)
