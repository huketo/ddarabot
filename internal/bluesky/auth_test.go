package bluesky

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth_CreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.createSession" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		json.NewEncoder(w).Encode(Session{
			AccessJwt:  "access-token",
			RefreshJwt: "refresh-token",
			DID:        "did:plc:test",
			Handle:     "test.bsky.social",
		})
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "test-password")
	session, err := auth.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.AccessJwt != "access-token" {
		t.Errorf("AccessJwt = %q, want %q", session.AccessJwt, "access-token")
	}
	if session.DID != "did:plc:test" {
		t.Errorf("DID = %q, want %q", session.DID, "did:plc:test")
	}
}

func TestAuth_RefreshSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.refreshSession" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer old-refresh-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer old-refresh-token")
		}
		json.NewEncoder(w).Encode(Session{
			AccessJwt:  "new-access-token",
			RefreshJwt: "new-refresh-token",
			DID:        "did:plc:test",
		})
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "test-password")
	auth.session = &Session{RefreshJwt: "old-refresh-token"}

	session, err := auth.RefreshSession(context.Background())
	if err != nil {
		t.Fatalf("RefreshSession() error = %v", err)
	}
	if session.AccessJwt != "new-access-token" {
		t.Errorf("AccessJwt = %q, want %q", session.AccessJwt, "new-access-token")
	}
}

func TestAuth_GetSession_AutoCreate(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(Session{
			AccessJwt:  "access-token",
			RefreshJwt: "refresh-token",
			DID:        "did:plc:test",
		})
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "test-password")

	session, err := auth.GetSession(context.Background())
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.AccessJwt != "access-token" {
		t.Errorf("AccessJwt = %q", session.AccessJwt)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	// Second call reuses session
	_, err = auth.GetSession(context.Background())
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should reuse)", callCount)
	}
}

func TestResolveDID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.identity.resolveHandle" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		handle := r.URL.Query().Get("handle")
		if handle != "test.bsky.social" {
			t.Errorf("handle = %q, want %q", handle, "test.bsky.social")
		}
		json.NewEncoder(w).Encode(map[string]string{"did": "did:plc:resolved123"})
	}))
	defer server.Close()

	did, err := ResolveDID(context.Background(), server.URL, "test.bsky.social")
	if err != nil {
		t.Fatalf("ResolveDID() error = %v", err)
	}
	if did != "did:plc:resolved123" {
		t.Errorf("DID = %q, want %q", did, "did:plc:resolved123")
	}
}
