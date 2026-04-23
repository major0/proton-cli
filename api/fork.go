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
// parent session. It pushes a fork request to the parent's host and pulls the
// child session from the target service's host.
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

	// Push: POST /auth/v4/sessions/forks on the parent's host.
	pushReq := ForkPushReq{
		ChildClientID: targetService.ClientID,
		Independent:   0,
		Payload:       ciphertext,
	}
	var pushResp ForkPushResp
	if err := parent.DoJSON(ctx, "POST", "/auth/v4/sessions/forks", pushReq, &pushResp); err != nil {
		return nil, nil, fmt.Errorf("%w: push: %w", ErrForkFailed, err)
	}

	slog.Debug("fork.push", "selector", pushResp.Selector, "service", targetService.Name)

	// Pull: GET /auth/v4/sessions/forks/<selector> on the child's host.
	pullResp, err := forkPull(ctx, parent, targetService.Host, pushResp.Selector)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: pull: %w", ErrForkFailed, err)
	}

	slog.Debug("fork.pull", "uid", pullResp.UID, "service", targetService.Name)

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
	if err := parent.DoJSON(ctx, "POST", "/auth/v4/sessions/forks", pushReq, &pushResp); err != nil {
		return nil, nil, fmt.Errorf("%w: push: %w", ErrForkFailed, err)
	}

	slog.Debug("fork.push", "selector", pushResp.Selector, "service", targetService.Name)

	pullResp, err := forkPull(ctx, parent, targetService.Host, pushResp.Selector)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: pull: %w", ErrForkFailed, err)
	}

	slog.Debug("fork.pull", "uid", pullResp.UID, "service", targetService.Name)

	decryptedBlob, err := DecryptForkBlob(pullResp.Payload, blobKey)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: decrypt blob: %w", ErrForkFailed, err)
	}

	child := SessionFromForkPull(pullResp, targetService, version)

	return child, []byte(decryptedBlob.KeyPassword), nil
}

// forkPull executes GET /auth/v4/sessions/forks/<selector> on the child host
// using the parent's auth headers and cookie jar.
func forkPull(ctx context.Context, parent *Session, childHost, selector string) (*ForkPullResp, error) {
	pullURL := childHost + "/auth/v4/sessions/forks/" + selector

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	// Use parent's auth headers — the fork selector is the auth mechanism,
	// but the parent's UID and token are required for the pull.
	req.Header.Set("x-pm-uid", parent.Auth.UID)
	req.Header.Set("Authorization", "Bearer "+parent.Auth.AccessToken)
	if parent.AppVersion != "" {
		req.Header.Set("x-pm-appversion", parent.AppVersion)
	}
	if parent.UserAgent != "" {
		req.Header.Set("User-Agent", parent.UserAgent)
	}
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Jar: parent.cookieJar}
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
// ServiceConfig. The version string is used to build the app version header.
func SessionFromForkPull(pull *ForkPullResp, svc ServiceConfig, version string) *Session {
	jar, _ := cookiejar.New(nil)
	appVersion := svc.AppVersion(version)

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
