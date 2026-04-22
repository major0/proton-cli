package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoSSE_AuthHeaders(t *testing.T) {
	var gotUID, gotAuth, gotAppVer, gotUA string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID = r.Header.Get("x-pm-uid")
		gotAuth = r.Header.Get("Authorization")
		gotAppVer = r.Header.Get("x-pm-appversion")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := testSession(t, "")
	s.AppVersion = "cli@2.0.0"
	s.UserAgent = "proton-cli/2.0"

	rc, err := s.DoSSE(context.Background(), srv.URL+"/ai/v1/chat", map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("DoSSE: %v", err)
	}
	_ = rc.Close()

	if gotUID != "test-uid-123" {
		t.Fatalf("x-pm-uid = %q, want %q", gotUID, "test-uid-123")
	}
	if gotAuth != "Bearer test-token-abc" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer test-token-abc")
	}
	if gotAppVer != "cli@2.0.0" {
		t.Fatalf("x-pm-appversion = %q, want %q", gotAppVer, "cli@2.0.0")
	}
	if gotUA != "proton-cli/2.0" {
		t.Fatalf("User-Agent = %q, want %q", gotUA, "proton-cli/2.0")
	}
}

func TestDoSSE_AcceptHeader(t *testing.T) {
	var gotAccept string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := testSession(t, "")
	rc, err := s.DoSSE(context.Background(), srv.URL+"/ai/v1/chat", nil)
	if err != nil {
		t.Fatalf("DoSSE: %v", err)
	}
	_ = rc.Close()

	if gotAccept != "text/event-stream" {
		t.Fatalf("Accept = %q, want %q", gotAccept, "text/event-stream")
	}
}

func TestDoSSE_NonSuccessReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Code":  2000,
			"Error": "access denied",
		})
	}))
	defer srv.Close()

	s := testSession(t, "")
	rc, err := s.DoSSE(context.Background(), srv.URL+"/ai/v1/chat", nil)
	if rc != nil {
		_ = rc.Close()
		t.Fatal("expected nil body on error")
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", apiErr.Status, http.StatusForbidden)
	}
	if apiErr.Code != 2000 {
		t.Fatalf("code = %d, want %d", apiErr.Code, 2000)
	}
	if apiErr.Message != "access denied" {
		t.Fatalf("message = %q, want %q", apiErr.Message, "access denied")
	}
}

func TestDoSSE_BodyPassthrough(t *testing.T) {
	ssePayload := "data: {\"type\":\"token_data\",\"content\":\"hello\"}\n\ndata: {\"type\":\"done\"}\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ssePayload))
	}))
	defer srv.Close()

	s := testSession(t, "")
	rc, err := s.DoSSE(context.Background(), srv.URL+"/ai/v1/chat", map[string]string{"prompt": "hi"})
	if err != nil {
		t.Fatalf("DoSSE: %v", err)
	}
	defer func() { _ = rc.Close() }()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != ssePayload {
		t.Fatalf("body = %q, want %q", string(got), ssePayload)
	}
}

func TestDoSSE_PostMethod(t *testing.T) {
	var gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := testSession(t, "")
	rc, err := s.DoSSE(context.Background(), srv.URL+"/ai/v1/chat", nil)
	if err != nil {
		t.Fatalf("DoSSE: %v", err)
	}
	_ = rc.Close()

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
}

func TestDoSSE_ContentTypeOnBody(t *testing.T) {
	var gotCT string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := testSession(t, "")

	// With body — should set Content-Type.
	rc, err := s.DoSSE(context.Background(), srv.URL+"/test", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("DoSSE with body: %v", err)
	}
	_ = rc.Close()
	if gotCT != "application/json" {
		t.Fatalf("Content-Type with body = %q, want %q", gotCT, "application/json")
	}

	// Without body — should not set Content-Type.
	gotCT = ""
	rc, err = s.DoSSE(context.Background(), srv.URL+"/test", nil)
	if err != nil {
		t.Fatalf("DoSSE without body: %v", err)
	}
	_ = rc.Close()
	if gotCT != "" {
		t.Fatalf("Content-Type without body = %q, want empty", gotCT)
	}
}

func TestDoSSE_BaseURLOverride(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := testSession(t, "")
	s.BaseURL = srv.URL

	rc, err := s.DoSSE(context.Background(), "/ai/v1/chat", nil)
	if err != nil {
		t.Fatalf("DoSSE with BaseURL: %v", err)
	}
	_ = rc.Close()

	if gotPath != "/ai/v1/chat" {
		t.Fatalf("path = %q, want %q", gotPath, "/ai/v1/chat")
	}
}

func TestDoSSE_NonSuccessNoBody(t *testing.T) {
	// Server returns 500 with no parseable JSON body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := testSession(t, "")
	rc, err := s.DoSSE(context.Background(), srv.URL+"/test", nil)
	if rc != nil {
		_ = rc.Close()
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", apiErr.Status, http.StatusInternalServerError)
	}
}
