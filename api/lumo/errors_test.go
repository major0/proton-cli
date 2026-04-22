package lumo

import (
	"errors"
	"testing"
)

func TestErrorSentinels(t *testing.T) {
	sentinels := []error{
		ErrStreamClosed,
		ErrRejected,
		ErrHarmful,
		ErrTimeout,
		ErrDecryptionFailed,
	}

	// Each sentinel must be distinct.
	for i := 0; i < len(sentinels); i++ {
		for j := i + 1; j < len(sentinels); j++ {
			if errors.Is(sentinels[i], sentinels[j]) {
				t.Errorf("sentinels[%d] == sentinels[%d]: %v", i, j, sentinels[i])
			}
		}
	}

	// Each sentinel must have a non-empty message.
	for _, s := range sentinels {
		if s.Error() == "" {
			t.Errorf("sentinel has empty message: %v", s)
		}
	}
}
