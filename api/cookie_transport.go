package api

import (
	"log/slog"
	"net/http"
	"strings"
)

// CookieTransport is an http.RoundTripper that converts Bearer auth to
// cookie auth. It strips the Authorization header added by Resty and relies
// on the cookie jar (set on the http.Client by Resty) to send AUTH-<uid>
// cookies instead.
//
// This transport is used after POST /core/v4/auth/cookies transitions a
// session from Bearer to cookie auth. After transition, Bearer tokens are
// invalid server-side — only the AUTH cookie authenticates requests.
//
// Usage:
//
//	ct := &CookieTransport{Base: http.DefaultTransport}
//	manager := proton.New(
//	    proton.WithTransport(ct),
//	    proton.WithCookieJar(jar),  // jar has AUTH-<uid> cookie
//	    ...
//	)
type CookieTransport struct {
	// Base is the underlying transport. If nil, http.DefaultTransport is used.
	Base http.RoundTripper
}

// RoundTrip strips the Authorization header and delegates to the base
// transport. The cookie jar on the http.Client sends AUTH-<uid> cookies
// automatically.
func (ct *CookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Strip Bearer auth — cookie auth only after transition.
	if auth := req.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			slog.Debug("cookieTransport: stripping Bearer header")
			req = req.Clone(req.Context())
			req.Header.Del("Authorization")
		}
	}

	base := ct.Base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(req)
}
