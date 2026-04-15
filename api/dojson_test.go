package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/ProtonMail/go-proton-api"
)

// testSession creates a minimal Session pointing at the given test server.
// It overrides proton.DefaultHostURL for the duration of the test.
func testSession(t *testing.T, _ string) *Session {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	return &Session{
		Auth: proton.Auth{
			UID:         "test-uid-123",
			AccessToken: "test-token-abc",
		},
		cookieJar: jar,
	}
}

func TestDoJSON_SuccessGet(t *testing.T) {
	type payload struct {
		Name string `json:"Name"`
		ID   int    `json:"ID"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("x-pm-uid") != "test-uid-123" {
			t.Fatalf("missing x-pm-uid header")
		}
		if r.Header.Get("Authorization") != "Bearer test-token-abc" {
			t.Fatalf("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Code": 1000,
			"Name": "test-share",
			"ID":   42,
		})
	}))
	defer srv.Close()

	s := testSession(t, srv.URL)
	var result payload
	err := s.DoJSON(context.Background(), "GET", srv.URL+"/test", nil, &result)
	if err != nil {
		t.Fatalf("DoJSON GET: %v", err)
	}
	if result.Name != "test-share" || result.ID != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDoJSON_SuccessPost(t *testing.T) {
	type reqBody struct {
		Email string `json:"Email"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("missing Content-Type header")
		}
		body, _ := io.ReadAll(r.Body)
		var req reqBody
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if req.Email != "user@example.com" {
			t.Fatalf("unexpected email: %s", req.Email)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000})
	}))
	defer srv.Close()

	s := testSession(t, srv.URL)
	err := s.DoJSON(context.Background(), "POST", srv.URL+"/invite", reqBody{Email: "user@example.com"}, nil)
	if err != nil {
		t.Fatalf("DoJSON POST: %v", err)
	}
}

func TestDoJSON_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Code":  2011,
			"Error": "Share not found",
		})
	}))
	defer srv.Close()

	s := testSession(t, srv.URL)
	err := s.DoJSON(context.Background(), "GET", srv.URL+"/test", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", apiErr.Status)
	}
	if apiErr.Code != 2011 {
		t.Fatalf("expected code 2011, got %d", apiErr.Code)
	}
	if apiErr.Message != "Share not found" {
		t.Fatalf("expected message 'Share not found', got %q", apiErr.Message)
	}
}

func TestDoJSON_AuthHeaders(t *testing.T) {
	var gotUID, gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID = r.Header.Get("x-pm-uid")
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000})
	}))
	defer srv.Close()

	s := testSession(t, srv.URL)
	_ = s.DoJSON(context.Background(), "GET", srv.URL+"/test", nil, nil)

	if gotUID != "test-uid-123" {
		t.Fatalf("x-pm-uid = %q, want %q", gotUID, "test-uid-123")
	}
	if gotAuth != "Bearer test-token-abc" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer test-token-abc")
	}
}

func TestDoJSON_CookiesAttached(t *testing.T) {
	// First request sets a cookie, second request should send it back.
	var gotCookie string

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			http.SetCookie(w, &http.Cookie{Name: "Session-Id", Value: "abc123", Path: "/"})
		} else {
			c, err := r.Cookie("Session-Id")
			if err != nil {
				gotCookie = ""
			} else {
				gotCookie = c.Value
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000})
	}))
	defer srv.Close()

	s := testSession(t, srv.URL)

	// First call — server sets cookie.
	_ = s.DoJSON(context.Background(), "GET", srv.URL+"/first", nil, nil)
	// Second call — cookie should be sent.
	_ = s.DoJSON(context.Background(), "GET", srv.URL+"/second", nil, nil)

	if gotCookie != "abc123" {
		t.Fatalf("cookie not attached on second request: got %q", gotCookie)
	}
}

func TestDoJSON_NilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			if len(body) > 0 {
				t.Fatalf("expected empty body for GET, got %d bytes", len(body))
			}
		}
		if r.Header.Get("Content-Type") != "" {
			t.Fatalf("Content-Type should not be set for nil body, got %q", r.Header.Get("Content-Type"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000})
	}))
	defer srv.Close()

	s := testSession(t, srv.URL)
	err := s.DoJSON(context.Background(), "GET", srv.URL+"/test", nil, nil)
	if err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
}

func TestDoJSON_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Code": 1000})
	}))
	defer srv.Close()

	s := testSession(t, srv.URL)
	err := s.DoJSON(context.Background(), "DELETE", srv.URL+"/member/123", nil, nil)
	if err != nil {
		t.Fatalf("DoJSON DELETE: %v", err)
	}
}
