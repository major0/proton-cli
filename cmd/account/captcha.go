package accountCmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/internal"
)

// formatHvURL builds the Proton CAPTCHA verification URL from HV details.
// Matches proton-bridge's hv.FormatHvURL.
func formatHvURL(details *proton.APIHVDetails) string {
	return fmt.Sprintf("https://verify.proton.me/?methods=%s&token=%s",
		strings.Join(details.Methods, ","),
		details.Token)
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(rawURL string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}
	return exec.Command(cmd, rawURL).Start() //nolint:gosec // URL is constructed internally
}

// SolveCaptcha opens the Proton CAPTCHA URL in the browser and waits for
// the user to solve it. The CAPTCHA is solved on Proton's servers — the
// backend marks the challenge token as verified. The caller retries the
// login with the same HV details and it succeeds.
func SolveCaptcha(hv *proton.APIHVDetails) {
	hvURL := formatHvURL(hv)

	fmt.Println("\nHuman verification required.")
	fmt.Printf("Opening CAPTCHA in browser: %s\n\n", hvURL)

	if err := openBrowser(hvURL); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser. Please open manually:\n  %s\n\n", hvURL)
	}

	fmt.Println("Solve the CAPTCHA in your browser, then press ENTER to continue.")
	_, _ = internal.UserPrompt("Press ENTER", false)
}

// SolveCaptchaNoBrowser prints the Proton CAPTCHA URL and waits for the
// user to solve it. For SSH/headless use where the CLI cannot open a browser.
func SolveCaptchaNoBrowser(hv *proton.APIHVDetails) {
	hvURL := formatHvURL(hv)

	fmt.Println("\nHuman verification required.")
	fmt.Println("Open this URL in your browser to solve the CAPTCHA:")
	fmt.Printf("\n  %s\n\n", hvURL)
	fmt.Println("Solve the CAPTCHA, then press ENTER to continue.")
	_, _ = internal.UserPrompt("Press ENTER", false)
}
