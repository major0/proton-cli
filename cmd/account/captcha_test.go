package accountCmd

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Browser mode: injectCallbackURL
// ---------------------------------------------------------------------------

func TestInjectCallbackURL(t *testing.T) {
	callback := "http://127.0.0.1:12345/captcha/callback"

	tests := []struct {
		name string
		html string
		want string
	}{
		{
			"replaces full callback URL",
			`<script>xhr.open("POST","https://verify.proton.me/captcha/v1/api/callback")</script>`,
			`<script>xhr.open("POST","` + callback + `")</script>`,
		},
		{
			"replaces base verify URL",
			`<img src="https://verify.proton.me/logo.png">`,
			`<img src="` + callback + `/logo.png">`,
		},
		{
			"replaces both occurrences",
			`<a href="https://verify.proton.me/captcha/v1/api/callback">link</a>` +
				`<img src="https://verify.proton.me/img.png">`,
			`<a href="` + callback + `">link</a>` +
				`<img src="` + callback + `/img.png">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := injectCallbackURL([]byte(tt.html), callback)
			if string(got) != tt.want {
				t.Errorf("injectCallbackURL:\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestInjectCallbackURL_NoMatch(t *testing.T) {
	html := []byte(`<html><body><p>No Proton URLs here</p></body></html>`)
	callback := "http://127.0.0.1:9999/captcha/callback"

	got := injectCallbackURL(html, callback)
	if !bytes.Equal(got, html) {
		t.Errorf("expected unchanged HTML, got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// No-browser mode: injectTokenDisplayJS
// ---------------------------------------------------------------------------

func TestInjectTokenDisplayJS_Head(t *testing.T) {
	html := []byte(`<html><head><title>CAPTCHA</title></head><body></body></html>`)
	got := injectTokenDisplayJS(html)

	headIdx := bytes.Index(got, []byte("</head>"))
	jsIdx := bytes.Index(got, []byte(captchaDisplayJS))
	if jsIdx < 0 {
		t.Fatal("JS not found in output")
	}
	if jsIdx >= headIdx {
		t.Errorf("JS inserted at %d, but </head> at %d — expected JS before </head>", jsIdx, headIdx)
	}
}

func TestInjectTokenDisplayJS_Body(t *testing.T) {
	// No </head> tag, only </body>.
	html := []byte(`<html><body><p>captcha</p></body></html>`)
	got := injectTokenDisplayJS(html)

	bodyIdx := bytes.Index(got, []byte("</body>"))
	jsIdx := bytes.Index(got, []byte(captchaDisplayJS))
	if jsIdx < 0 {
		t.Fatal("JS not found in output")
	}
	if jsIdx >= bodyIdx {
		t.Errorf("JS inserted at %d, but </body> at %d — expected JS before </body>", jsIdx, bodyIdx)
	}
}

func TestInjectTokenDisplayJS_Fallback(t *testing.T) {
	// Neither </head> nor </body>.
	html := []byte(`<div>bare fragment</div>`)
	got := injectTokenDisplayJS(html)

	if !bytes.HasPrefix(got, html) {
		t.Error("original HTML not preserved as prefix")
	}
	if !bytes.HasSuffix(got, []byte(captchaDisplayJS)) {
		t.Error("JS not appended to end")
	}
}

func TestInjectTokenDisplayJS_ContainsScript(t *testing.T) {
	// Verify the constant contains the expected interception patterns.
	if !strings.Contains(captchaDisplayJS, "XMLHttpRequest.prototype.open") {
		t.Error("captchaDisplayJS missing XMLHttpRequest override")
	}
	if !strings.Contains(captchaDisplayJS, "XMLHttpRequest.prototype.send") {
		t.Error("captchaDisplayJS missing XMLHttpRequest.send override")
	}
	if !strings.Contains(captchaDisplayJS, "window.fetch") {
		t.Error("captchaDisplayJS missing fetch override")
	}
	if !strings.Contains(captchaDisplayJS, "Verification Token") {
		t.Error("captchaDisplayJS missing token display heading")
	}
}

// ---------------------------------------------------------------------------
// Helper: insertAt
// ---------------------------------------------------------------------------

func TestInsertAt(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		data   string
		offset int
		want   string
	}{
		{"beginning", "hello", "XX", 0, "XXhello"},
		{"middle", "hello", "XX", 2, "heXXllo"},
		{"end", "hello", "XX", 5, "helloXX"},
		{"empty src", "", "XX", 0, "XX"},
		{"empty data", "hello", "", 3, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := insertAt([]byte(tt.src), []byte(tt.data), tt.offset)
			if string(got) != tt.want {
				t.Errorf("insertAt(%q, %q, %d) = %q, want %q",
					tt.src, tt.data, tt.offset, got, tt.want)
			}
		})
	}
}
