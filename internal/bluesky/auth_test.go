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

func TestAuth_RefreshSession_AfterInvalidate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.refreshSession" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer original-refresh-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer original-refresh-token")
		}
		json.NewEncoder(w).Encode(Session{
			AccessJwt:  "refreshed-access-token",
			RefreshJwt: "refreshed-refresh-token",
			DID:        "did:plc:test",
		})
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "test-password")
	auth.session = &Session{
		AccessJwt:  "old-access-token",
		RefreshJwt: "original-refresh-token",
		DID:        "did:plc:test",
	}
	// Preserve the refresh token, then invalidate the session
	auth.lastRefreshJwt = "original-refresh-token"
	auth.InvalidateSession()

	if auth.session != nil {
		t.Fatal("session should be nil after InvalidateSession()")
	}

	// RefreshSession should still work using the preserved lastRefreshJwt
	session, err := auth.RefreshSession(context.Background())
	if err != nil {
		t.Fatalf("RefreshSession() after invalidate error = %v", err)
	}
	if session.AccessJwt != "refreshed-access-token" {
		t.Errorf("AccessJwt = %q, want %q", session.AccessJwt, "refreshed-access-token")
	}
	if session.RefreshJwt != "refreshed-refresh-token" {
		t.Errorf("RefreshJwt = %q, want %q", session.RefreshJwt, "refreshed-refresh-token")
	}
}

func TestAuth_GetSession_RefreshBeforeCreate(t *testing.T) {
	var calledPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledPaths = append(calledPaths, r.URL.Path)
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			json.NewEncoder(w).Encode(Session{
				AccessJwt:  "refreshed-access-token",
				RefreshJwt: "refreshed-refresh-token",
				DID:        "did:plc:test",
			})
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(Session{
				AccessJwt:  "created-access-token",
				RefreshJwt: "created-refresh-token",
				DID:        "did:plc:test",
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "test-password")
	// Set lastRefreshJwt but leave session nil to simulate a recovered state
	auth.lastRefreshJwt = "saved-refresh-token"

	session, err := auth.GetSession(context.Background())
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}

	// Verify refresh was attempted first (not create)
	if len(calledPaths) != 1 {
		t.Fatalf("expected 1 server call, got %d: %v", len(calledPaths), calledPaths)
	}
	if calledPaths[0] != "/xrpc/com.atproto.server.refreshSession" {
		t.Errorf("first call = %q, want refresh endpoint", calledPaths[0])
	}
	if session.AccessJwt != "refreshed-access-token" {
		t.Errorf("AccessJwt = %q, want %q", session.AccessJwt, "refreshed-access-token")
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
