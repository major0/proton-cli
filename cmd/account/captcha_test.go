package accountCmd

import (
	"testing"

	proton "github.com/ProtonMail/go-proton-api"
)

func TestFormatHvURL(t *testing.T) {
	tests := []struct {
		name    string
		details *proton.APIHVDetails
		want    string
	}{
		{
			"single method",
			&proton.APIHVDetails{Methods: []string{"captcha"}, Token: "abc123"},
			"https://verify.proton.me/?methods=captcha&token=abc123",
		},
		{
			"multiple methods",
			&proton.APIHVDetails{Methods: []string{"captcha", "sms"}, Token: "xyz"},
			"https://verify.proton.me/?methods=captcha,sms&token=xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatHvURL(tt.details)
			if got != tt.want {
				t.Errorf("formatHvURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
