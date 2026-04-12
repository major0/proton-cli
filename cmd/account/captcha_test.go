package accountCmd

import (
	"testing"
)

func TestCaptchaURL(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			"basic token",
			"abc123",
			"https://drive-api.proton.me/core/v4/captcha?Token=abc123&ForceWebMessaging=1",
		},
		{
			"token with special chars",
			"abc-123_XYZ",
			"https://drive-api.proton.me/core/v4/captcha?Token=abc-123_XYZ&ForceWebMessaging=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captchaURL(tt.token)
			if got != tt.want {
				t.Errorf("captchaURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
