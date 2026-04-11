package accountCmd

import (
	"errors"
	"fmt"
	"testing"

	proton "github.com/ProtonMail/go-proton-api"
)

func TestHasCaptchaMethod(t *testing.T) {
	tests := []struct {
		name    string
		methods []string
		want    bool
	}{
		{"captcha only", []string{"captcha"}, true},
		{"captcha and sms", []string{"captcha", "sms"}, true},
		{"sms only", []string{"sms"}, false},
		{"email only", []string{"email"}, false},
		{"empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasCaptchaMethod(tt.methods); got != tt.want {
				t.Errorf("hasCaptchaMethod(%v) = %v, want %v", tt.methods, got, tt.want)
			}
		})
	}
}

func TestHVErrorDetection(t *testing.T) {
	t.Run("APIError 9001 is HV error", func(t *testing.T) {
		apiErr := &proton.APIError{Code: proton.HumanVerificationRequired}

		var target *proton.APIError
		if !errors.As(apiErr, &target) {
			t.Fatal("errors.As failed for *proton.APIError with code 9001")
		}
		if !target.IsHVError() {
			t.Error("IsHVError() returned false for code 9001")
		}
	})

	t.Run("APIError 8002 is not HV error", func(t *testing.T) {
		apiErr := &proton.APIError{Code: proton.PasswordWrong}

		var target *proton.APIError
		if !errors.As(apiErr, &target) {
			t.Fatal("errors.As failed for *proton.APIError with code 8002")
		}
		if target.IsHVError() {
			t.Error("IsHVError() returned true for code 8002")
		}
	})

	t.Run("plain error is not APIError", func(t *testing.T) {
		err := fmt.Errorf("some error")

		var target *proton.APIError
		if errors.As(err, &target) {
			t.Error("errors.As matched a plain error as *proton.APIError")
		}
	})

	t.Run("wrapped APIError 9001 detected via errors.As", func(t *testing.T) {
		apiErr := &proton.APIError{Code: proton.HumanVerificationRequired, Message: "HV required"}
		wrapped := fmt.Errorf("login failed: %w", apiErr)

		var target *proton.APIError
		if !errors.As(wrapped, &target) {
			t.Fatal("errors.As failed to unwrap *proton.APIError from wrapped error")
		}
		if !target.IsHVError() {
			t.Error("IsHVError() returned false for unwrapped code 9001")
		}
	})
}
