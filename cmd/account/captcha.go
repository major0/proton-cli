package accountCmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ProtonMail/go-proton-api"
	cli "github.com/major0/proton-cli/cmd"
	"github.com/major0/proton-cli/internal"
)

// fetchCaptchaHTML creates a temporary Manager, fetches the CAPTCHA HTML page
// for the given HV token, and closes the manager. Returns the raw HTML bytes.
func fetchCaptchaHTML(ctx context.Context, opts []proton.Option, hv *proton.APIHVDetails) ([]byte, error) {
	manager := proton.New(opts...)
	defer manager.Close()

	html, err := manager.GetCaptcha(ctx, hv.Token)
	if err != nil {
		return nil, fmt.Errorf("fetching captcha page: %w", err)
	}

	return html, nil
}

// openBrowser opens the given URL in the user's default browser.
func openBrowser(url string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}

	return exec.Command(cmd, url).Start() //nolint:gosec // URL is constructed internally, not user-supplied
}

// writeTempHTML writes HTML bytes to a temporary file and returns its path.
func writeTempHTML(html []byte) (string, error) {
	dir := os.TempDir()
	path := filepath.Join(dir, "proton-captcha.html")

	if err := os.WriteFile(path, html, 0600); err != nil {
		return "", fmt.Errorf("writing captcha temp file: %w", err)
	}

	return path, nil
}

// SolveCaptcha opens a CAPTCHA challenge in the user's browser and waits
// for the solved token via a local HTTP callback listener.
func SolveCaptcha(ctx context.Context, opts []proton.Option, hv *proton.APIHVDetails) (string, error) {
	html, err := fetchCaptchaHTML(ctx, opts, hv)
	if err != nil {
		return "", err
	}

	// Start local listener on a random port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("starting captcha listener: %w", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.Addr().(*net.TCPAddr)
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/captcha/callback", addr.Port)

	slog.Debug("captcha.listener", "callback", callbackURL)

	// Inject the callback URL into the CAPTCHA HTML.
	// The Proton CAPTCHA page posts the solved token to the API URL.
	// Replace the API base URL with our local listener so we intercept the POST.
	// TODO: The exact replacement target may need adjustment after testing
	// with real CAPTCHA pages. This covers the common pattern where the
	// page references the API host for the callback.
	modified := injectCallbackURL(html, callbackURL)

	tmpPath, err := writeTempHTML(modified)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpPath) }()

	tokenCh := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/captcha/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "<html><body><p>Verification complete. You may close this window.</p></body></html>")

		select {
		case tokenCh <- string(body):
		default:
		}
	})

	// Handle CORS preflight for the callback.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: cli.Timeout} //nolint:gosec // local-only listener
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("captcha.server", "error", err)
		}
	}()
	defer func() { _ = server.Close() }()

	// Open the CAPTCHA page in the browser.
	fileURL := "file://" + tmpPath
	if err := openBrowser(fileURL); err != nil {
		// Non-fatal: user can navigate manually.
		slog.Warn("captcha.browser", "error", err)
		fmt.Fprintf(os.Stderr, "Could not open browser. Please open manually:\n%s\n", fileURL)
	}

	// Wait for the token or context cancellation.
	select {
	case token := <-tokenCh:
		return token, nil
	case <-ctx.Done():
		return "", fmt.Errorf("human verification not completed: %w", ctx.Err())
	}
}

// injectCallbackURL replaces the Proton API base URL in the CAPTCHA HTML
// with the local listener callback URL so the solved token is posted locally.
// TODO: The replacement target may need adjustment after testing with real
// CAPTCHA pages from the Proton API.
func injectCallbackURL(html []byte, callbackURL string) []byte {
	// The CAPTCHA page typically posts to the Proton API host.
	// Replace known API URL patterns with our local callback.
	targets := []string{
		"https://verify.proton.me/captcha/v1/api/callback",
		"https://verify.proton.me",
	}

	result := html
	for _, target := range targets {
		result = bytes.ReplaceAll(result, []byte(target), []byte(callbackURL))
	}

	return result
}

// captchaDisplayJS is a JavaScript snippet injected into the CAPTCHA HTML for
// no-browser mode. It intercepts the CAPTCHA completion callback and displays
// the solved token in a copyable element instead of posting it.
const captchaDisplayJS = `<script>
(function() {
    // Override XMLHttpRequest to intercept the token POST.
    var origOpen = XMLHttpRequest.prototype.open;
    var origSend = XMLHttpRequest.prototype.send;
    XMLHttpRequest.prototype.open = function(method, url) {
        this._captchaMethod = method;
        this._captchaURL = url;
        return origOpen.apply(this, arguments);
    };
    XMLHttpRequest.prototype.send = function(body) {
        if (this._captchaMethod === 'POST' && body) {
            var token = body;
            if (typeof body === 'object') {
                try { token = JSON.stringify(body); } catch(e) { token = String(body); }
            }
            var container = document.createElement('div');
            container.style.cssText = 'position:fixed;top:0;left:0;right:0;padding:20px;background:#fff;z-index:999999;text-align:center;font-family:monospace;';
            container.innerHTML = '<h2>Verification Token</h2><p>Copy the token below and paste it into the CLI:</p>' +
                '<pre style="background:#f0f0f0;padding:10px;word-break:break-all;user-select:all;">' + token + '</pre>';
            document.body.insertBefore(container, document.body.firstChild);
            return;
        }
        return origSend.apply(this, arguments);
    };

    // Also intercept fetch for modern CAPTCHA implementations.
    var origFetch = window.fetch;
    window.fetch = function(url, opts) {
        if (opts && opts.method && opts.method.toUpperCase() === 'POST' && opts.body) {
            var token = opts.body;
            if (typeof token === 'object') {
                try { token = JSON.stringify(token); } catch(e) { token = String(token); }
            }
            var container = document.createElement('div');
            container.style.cssText = 'position:fixed;top:0;left:0;right:0;padding:20px;background:#fff;z-index:999999;text-align:center;font-family:monospace;';
            container.innerHTML = '<h2>Verification Token</h2><p>Copy the token below and paste it into the CLI:</p>' +
                '<pre style="background:#f0f0f0;padding:10px;word-break:break-all;user-select:all;">' + token + '</pre>';
            document.body.insertBefore(container, document.body.firstChild);
            return Promise.resolve(new Response('ok'));
        }
        return origFetch.apply(this, arguments);
    };
})();
</script>`

// SolveCaptchaNoBrowser writes the CAPTCHA HTML to a temp file with injected
// JS that displays the solved token, prints the file URL, and prompts the
// user to paste the token. For SSH/headless use.
func SolveCaptchaNoBrowser(ctx context.Context, opts []proton.Option, hv *proton.APIHVDetails) (string, error) {
	html, err := fetchCaptchaHTML(ctx, opts, hv)
	if err != nil {
		return "", err
	}

	// Inject token-display JS before the closing </head> or </body> tag.
	modified := injectTokenDisplayJS(html)

	tmpPath, err := writeTempHTML(modified)
	if err != nil {
		return "", err
	}
	// Don't remove the temp file — user needs to open it manually.

	fileURL := "file://" + tmpPath
	fmt.Printf("Open this URL in your browser to solve the CAPTCHA:\n%s\n\n", fileURL)

	token, err := internal.UserPrompt("Verification token", false)
	if err != nil {
		return "", fmt.Errorf("reading verification token: %w", err)
	}

	if token == "" {
		return "", fmt.Errorf("human verification not completed: empty token")
	}

	return token, nil
}

// injectTokenDisplayJS inserts the token-display JavaScript into the CAPTCHA
// HTML. It places the script before </head> if present, otherwise before </body>.
func injectTokenDisplayJS(html []byte) []byte {
	jsBytes := []byte(captchaDisplayJS)

	if idx := bytes.Index(html, []byte("</head>")); idx >= 0 {
		return insertAt(html, jsBytes, idx)
	}

	if idx := bytes.Index(html, []byte("</body>")); idx >= 0 {
		return insertAt(html, jsBytes, idx)
	}

	// Fallback: append to end.
	return append(html, jsBytes...)
}

// insertAt inserts data into src at the given byte offset.
func insertAt(src, data []byte, offset int) []byte {
	result := make([]byte, len(src)+len(data))
	copy(result, src[:offset])
	copy(result[offset:], data)
	copy(result[offset+len(data):], src[offset:])
	return result
}
