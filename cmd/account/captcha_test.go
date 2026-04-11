package accountCmd

import (
	"testing"
)

func TestExtractTokenPrefix(t *testing.T) {
	tests := []struct {
		name  string
		html  string
		token string
		want  string
	}{
		{
			"standard prefix",
			`function tokenCallback(response) { return sendToken('LYxTbWWYSZK3JrTMrBkLWaVK'+'7HHmCTeFmwb/DnN7OpMdW6qL'+response); }`,
			"test-token",
			"LYxTbWWYSZK3JrTMrBkLWaVK7HHmCTeFmwb/DnN7OpMdW6qL",
		},
		{
			"single string prefix",
			`function tokenCallback(response) { return sendToken('ABCDEF'+response); }`,
			"test-token",
			"ABCDEF",
		},
		{
			"no match",
			`<html><body>no tokenCallback here</body></html>`,
			"test-token",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTokenPrefix(tt.html, tt.token)
			if got != tt.want {
				t.Errorf("extractTokenPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
