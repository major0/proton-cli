package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	proton "github.com/ProtonMail/go-proton-api"
)

// ErrForkFailed indicates that the session fork protocol failed.
var ErrForkFailed = errors.New("session fork failed")

// ForkPushReq is the request body for POST /auth/v4/sessions/forks.
type ForkPushReq struct {
	ChildClientID string `json:"ChildClientID"`
	Independent   int    `json:"Independent"`
	Payload       string `json:"Payload,omitempty"`
}

// ForkPushResp is the response from POST /auth/v4/sessions/forks.
type ForkPushResp struct {
	Code     int    `json:"Code"`
	Selector string `json:"Selector"`
}

// ForkPullResp is the response from GET /auth/v4/sessions/forks/<selector>.
type ForkPullResp struct {
	Code         int      `json:"Code"`
	UID          string   `json:"UID"`
	AccessToken  string   `json:"AccessToken"`
	RefreshToken string   `json:"RefreshToken"`
	Payload      string   `json:"Payload,omitempty"`
	Scopes       []string `json:"Scopes,omitempty"`
}

// ForkSession creates a child session for targetService by forking from the
// parent session. Both push and pull go to the target service's host — the
// parent's auth headers authenticate the push, and the selector authenticates
// the pull.
//
// The parent session must be authenticated (valid UID/AccessToken). The child
// session is returned with BaseURL and AppVersion set to the target service's
// values. The decrypted SaltedKeyPass from the fork blob is returned as the
// second value.
func ForkSession(ctx context.Context, parent *Session, targetService ServiceConfig, version string) (*Session, []byte, error) {
	// Encrypt the parent's SaltedKeyPass into a fork blob.
	blob := &ForkBlob{
		Type:        "default",
		KeyPassword: parent.Auth.UID, // placeholder; callers set this from SessionConfig
	}

	ciphertext, blobKey, err := EncryptForkBlob(blob)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: encrypt blob: %w", ErrForkFailed, err)
	}

	// Push: POST /auth/v4/sessions/forks on the parent's (account) host.
	// The ChildClientID tells the server which scopes to grant for the
	// child session. The pull goes to the target service host.
	pushReq := ForkPushReq{
		ChildClientID: targetService.ClientID,
		Independent:   0,
		Payload:       ciphertext,
	}
	var pushResp ForkPushResp

	// Check for AUTH-* cookie presence.
	hasAuthCookie := false
	if pushURL, err := url.Parse(parent.BaseURL); err == nil {
		for _, c := range parent.cookieJar.Cookies(pushURL) {
			if strings.HasPrefix(c.Name, "AUTH-") {
				hasAuthCookie = true
				break
			}
		}
	}
	if !hasAuthCookie {
		slog.Warn("fork.push: no AUTH-* cookie in jar — child session may have restricted scopes", "service", targetService.Name)
	}

	if err := parent.DoJSONCookie(ctx, "POST", "/auth/v4/sessions/forks", pushReq, &pushResp); err != nil {
		return nil, nil, fmt.Errorf("%w: push: %w", ErrForkFailed, err)
	}

	// Log cookies after push (the push response may set new cookies).
	if pushURL, err := url.Parse(parent.BaseURL); err == nil {
		postPushCookies := parent.cookieJar.Cookies(pushURL)
		names := make([]string, len(postPushCookies))
		for i, c := range postPushCookies {
			names[i] = c.Name
		}
		slog.Debug("fork.push.cookies_after", "host", parent.BaseURL, "cookies", names)
	}

	slog.Debug("fork.push", "selector", pushResp.Selector, "service", targetService.Name, "child_client_id", targetService.ClientID, "push_host", parent.BaseURL)

	// Pull: GET /auth/v4/sessions/forks/<selector> on the target service host.
	pullResp, err := forkPull(ctx, parent, targetService.Host, pushResp.Selector, targetService.AppVersion(""))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: pull: %w", ErrForkFailed, err)
	}

	slog.Debug("fork.pull", "uid", pullResp.UID, "service", targetService.Name, "scopes", pullResp.Scopes)

	// Decrypt the fork blob from the pull response.
	decryptedBlob, err := DecryptForkBlob(pullResp.Payload, blobKey)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: decrypt blob: %w", ErrForkFailed, err)
	}

	// Build the child session.
	child := SessionFromForkPull(pullResp, targetService, version)

	return child, []byte(decryptedBlob.KeyPassword), nil
}

// ForkSessionWithKeyPass creates a child session, encrypting the given
// SaltedKeyPass in the fork blob instead of using the parent's UID.
func ForkSessionWithKeyPass(ctx context.Context, parent *Session, targetService ServiceConfig, version string, keyPass []byte) (*Session, []byte, error) {
	blob := &ForkBlob{
		Type:        "default",
		KeyPassword: string(keyPass),
	}

	ciphertext, blobKey, err := EncryptForkBlob(blob)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: encrypt blob: %w", ErrForkFailed, err)
	}

	pushReq := ForkPushReq{
		ChildClientID: targetService.ClientID,
		Independent:   0,
		Payload:       ciphertext,
	}
	var pushResp ForkPushResp

	// Check for AUTH-* cookie presence.
	hasAuthCookie := false
	if pushURL, err := url.Parse(parent.BaseURL); err == nil {
		for _, c := range parent.cookieJar.Cookies(pushURL) {
			if strings.HasPrefix(c.Name, "AUTH-") {
				hasAuthCookie = true
				break
			}
		}
	}
	if !hasAuthCookie {
		slog.Warn("fork.push: no AUTH-* cookie in jar — child session may have restricted scopes", "service", targetService.Name)
	}

	if err := parent.DoJSONCookie(ctx, "POST", "/auth/v4/sessions/forks", pushReq, &pushResp); err != nil {
		return nil, nil, fmt.Errorf("%w: push: %w", ErrForkFailed, err)
	}

	slog.Debug("fork.push", "selector", pushResp.Selector, "service", targetService.Name, "child_client_id", targetService.ClientID, "push_host", parent.BaseURL)

	// Pull from the target service host.
	pullResp, err := forkPull(ctx, parent, targetService.Host, pushResp.Selector, targetService.AppVersion(""))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: pull: %w", ErrForkFailed, err)
	}

	slog.Debug("fork.pull", "uid", pullResp.UID, "service", targetService.Name, "scopes", pullResp.Scopes)

	decryptedBlob, err := DecryptForkBlob(pullResp.Payload, blobKey)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: decrypt blob: %w", ErrForkFailed, err)
	}

	child := SessionFromForkPull(pullResp, targetService, version)

	return child, []byte(decryptedBlob.KeyPassword), nil
}

// forkPull executes GET /auth/v4/sessions/forks/<selector> on the target
// service host. The pull is unauthenticated (no Bearer token) — the
// selector in the URL path is the credential. Session cookies from the
// parent's jar are propagated to the target host for correlation.
func forkPull(ctx context.Context, parent *Session, host, selector, appVersion string) (*ForkPullResp, error) {
	pullURL := host + "/auth/v4/sessions/forks/" + selector

	slog.Debug("fork.pull.request", "url", pullURL, "appversion", appVersion)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	if appVersion != "" {
		req.Header.Set("x-pm-appversion", appVersion)
	}
	if parent.UserAgent != "" {
		req.Header.Set("User-Agent", parent.UserAgent)
	}
	req.Header.Set("Accept", "application/json")

	// Propagate the Session-Id cookie from the parent's jar to the pull jar.
	// The Proton API uses Session-Id to correlate the pull with the push.
	// Only Session-Id is propagated — AUTH-* cookies must not be sent on
	// the pull (the browser's pull is a fresh page load with no auth).
	pullJar, _ := cookiejar.New(nil)
	protonHosts := []*url.URL{
		{Scheme: "https", Host: "account-api.proton.me"},
		{Scheme: "https", Host: "account.proton.me"},
		{Scheme: "https", Host: "mail.proton.me"},
	}
	targetURL, _ := url.Parse(host)
	for _, srcURL := range protonHosts {
		for _, c := range parent.cookieJar.Cookies(srcURL) {
			if c.Name == "Session-Id" {
				pullJar.SetCookies(targetURL, []*http.Cookie{c})
			}
		}
	}

	pullCookies := pullJar.Cookies(targetURL)
	cookieNames := make([]string, len(pullCookies))
	for i, c := range pullCookies {
		cookieNames[i] = c.Name + "=" + c.Value[:min(8, len(c.Value))] + "..."
	}
	slog.Debug("fork.pull.cookies", "host", host, "cookies", cookieNames)

	httpClient := &http.Client{Jar: pullJar}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", pullURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	if envelope.Code != 1000 {
		slog.Debug("fork.pull.error", "url", pullURL, "status", resp.StatusCode, "code", envelope.Code, "message", envelope.Error)
		return nil, &Error{
			Status:  resp.StatusCode,
			Code:    envelope.Code,
			Message: envelope.Error,
		}
	}

	var pullResp ForkPullResp
	if err := json.Unmarshal(body, &pullResp); err != nil {
		return nil, fmt.Errorf("unmarshal pull response: %w", err)
	}

	return &pullResp, nil
}

// SessionFromForkPull constructs a Session from a ForkPullResp and
// ServiceConfig. The version string is passed through for backward
// compatibility but the service's own app version is used for all requests.
func SessionFromForkPull(pull *ForkPullResp, svc ServiceConfig, _ string) *Session {
	jar, _ := cookiejar.New(nil)
	appVersion := svc.AppVersion("")

	manager := proton.New(
		proton.WithHostURL(svc.Host),
		proton.WithAppVersion(appVersion),
		proton.WithCookieJar(jar),
	)

	client := manager.NewClient(pull.UID, pull.AccessToken, pull.RefreshToken)

	return &Session{
		Client: client,
		Auth: proton.Auth{
			UID:          pull.UID,
			AccessToken:  pull.AccessToken,
			RefreshToken: pull.RefreshToken,
		},
		BaseURL:    svc.Host,
		AppVersion: appVersion,
		manager:    manager,
		cookieJar:  jar,
	}
}
