package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ProtonMail/go-proton-api"
)

// TestSaltKeyPass verifies SaltKeyPass error paths using a mock API server.
// SaltKeyPass calls client.GetUser and client.GetSalts — we test the error
// propagation for each.
func TestSaltKeyPass(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "GetUser fails",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				// All requests return 401 — go-proton-api wraps this as
				// a Resty error with the HTTP status.
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Code":  401,
					"Error": "Unauthorized",
				})
			},
			wantErr: "401",
		},
		{
			name: "GetSalts fails",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/users") {
					_ = json.NewEncoder(w).Encode(map[string]any{
						"Code": 1000,
						"User": map[string]any{
							"ID":   "user-1",
							"Name": "test",
							"Keys": []map[string]any{
								{"ID": "key-1", "Primary": 1, "PrivateKey": ""},
							},
						},
					})
					return
				}
				// GetSalts fails with 500.
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"Code":  5000,
					"Error": "Internal Server Error",
				})
			},
			wantErr: "500",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			mgr := proton.New(proton.WithHostURL(srv.URL))
			defer mgr.Close()
			client := mgr.NewClient("uid", "token", "refresh")

			_, err := SaltKeyPass(context.Background(), client, []byte("password"))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
