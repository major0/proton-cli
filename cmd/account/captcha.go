package accountCmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ProtonMail/go-proton-api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/major0/proton-cli/internal"
)

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

// extractTokenPrefix extracts the server-generated token prefix from the
// CAPTCHA wrapper HTML. Proton's tokenCallback prepends this prefix to the
// CAPTCHA response before sending it via postMessage.
//
// The pattern in the HTML is:
//
//	function tokenCallback(response) { return sendToken('PREFIX'+response); }
//
// where sendToken prepends "captchaToken:" to produce the final token.
func extractTokenPrefix(html, _ string) string {
	// Look for: sendToken('...'+response)
	// The prefix may be split: sendToken('abc'+'def'+response)
	marker := "return sendToken('"
	idx := strings.Index(html, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	// Find "+response)" which ends the prefix portion.
	endMarker := "+response)"
	end := strings.Index(html[start:], endMarker)
	if end < 0 {
		return ""
	}
	raw := html[start : start+end]
	// Remove JS string concatenation artifacts: '+' between segments.
	raw = strings.ReplaceAll(raw, "'+'", "")
	// Strip any trailing quote.
	raw = strings.TrimRight(raw, "'")
	return raw
}

// SolveCaptcha starts a local HTTP server that acts as the web application
// embedding Proton's CAPTCHA. The browser loads our page, which contains an
// iframe pointing to verify.proton.me. When the user solves the CAPTCHA, the
// iframe posts the token via postMessage to our parent page, which forwards
// it to our local callback endpoint.
func SolveCaptcha(ctx context.Context, opts []proton.Option, hv *proton.APIHVDetails) (string, error) {
	// Fetch the wrapper HTML to extract the token prefix that Proton's
	// tokenCallback prepends to the CAPTCHA response.
	manager := proton.New(opts...)
	wrapperHTML, err := manager.GetCaptcha(ctx, hv.Token)
	manager.Close()
	if err != nil {
		return "", fmt.Errorf("fetching captcha page: %w", err)
	}
	tokenPrefix := extractTokenPrefix(string(wrapperHTML), hv.Token)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("starting captcha listener: %w", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.Addr().(*net.TCPAddr)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", addr.Port)
	slog.Debug("captcha.listener", "url", baseURL)

	tokenCh := make(chan string, 1)
	mux := http.NewServeMux()

	// Callback endpoint — receives the token from our page's JS.
	mux.HandleFunc("/captcha/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<html><body><h2 style="text-align:center;margin-top:40px">Verification complete. You may close this window.</h2></body></html>`)
		select {
		case tokenCh <- string(body):
		default:
		}
	})

	// Root — our wrapper page with the CAPTCHA iframe.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, captchaPageHTML, hv.Token, hv.Token, tokenPrefix, baseURL)
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: cli.Timeout} //nolint:gosec // local-only
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("captcha.server", "error", err)
		}
	}()
	defer func() { _ = server.Close() }()

	if err := openBrowser(baseURL); err != nil {
		slog.Warn("captcha.browser", "error", err)
		fmt.Fprintf(os.Stderr, "Could not open browser. Please open:\n  %s\n", baseURL)
	} else {
		fmt.Println("CAPTCHA opened in browser. Solve it to continue.")
	}

	select {
	case token := <-tokenCh:
		return token, nil
	case <-ctx.Done():
		return "", fmt.Errorf("human verification not completed: %w", ctx.Err())
	}
}

// captchaPageHTML is our local "web app" page. It embeds the Proton CAPTCHA
// in an iframe loaded directly from verify.proton.me (no proxy). The iframe
// posts the solved token via postMessage('*'), which our page receives and
// forwards to the local /captcha/callback endpoint.
//
// Format args: [1] iframe token param, [2] captcha token, [3] prefix, [4] base URL
const captchaPageHTML = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Proton CAPTCHA</title></head>
<body style="margin:0;display:flex;justify-content:center;align-items:start;min-height:100vh;background:#f5f5f5;padding-top:40px">
<div>
<iframe src="https://verify.proton.me/captcha/v1/assets/?purpose=login&token=%[1]s"
  width="500" height="580" style="border:none"></iframe>
</div>
<script>
window.addEventListener('message', function(event) {
  if (!event.data) return;
  if (event.data.type === 'proton_captcha' && event.data.token) {
    var fullToken = '%[2]s' + ':' + '%[3]s' + event.data.token;
    var xhr = new XMLHttpRequest();
    xhr.open('POST', '%[4]s/captcha/callback', true);
    xhr.setRequestHeader('Content-Type', 'text/plain');
    xhr.onload = function() {
      document.body.innerHTML = '<h2 style="text-align:center;margin-top:40px">Verification complete. You may close this window.</h2>';
    };
    xhr.send(fullToken);
  }
});
</script>
</body></html>`

// SolveCaptchaNoBrowser prints the Proton CAPTCHA URL and prompts the user
// to paste the solved token. For SSH/headless use where the CLI cannot open
// a local browser.
func SolveCaptchaNoBrowser(_ context.Context, _ []proton.Option, hv *proton.APIHVDetails) (string, error) {
	webURL := fmt.Sprintf("https://verify.proton.me/?methods=captcha&token=%s", hv.Token)

	fmt.Println("Human verification required.")
	fmt.Println("Open this URL in your browser to solve the CAPTCHA:")
	fmt.Printf("\n  %s\n\n", webURL)
	fmt.Println("After solving, copy the verification token and paste it below.")

	token, err := internal.UserPrompt("Verification token", false)
	if err != nil {
		return "", fmt.Errorf("reading verification token: %w", err)
	}

	if token == "" {
		return "", fmt.Errorf("human verification not completed: empty token")
	}

	return token, nil
}
