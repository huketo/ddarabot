package bluesky

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildFacets(t *testing.T) {
	text := "Hello world\n\n🌐 Translated by #DDaraBot"
	facets := BuildHashtagFacets(text, "DDaraBot")

	if len(facets) != 1 {
		t.Fatalf("len(facets) = %d, want 1", len(facets))
	}

	f := facets[0]
	textBytes := []byte(text)
	tag := string(textBytes[f.Index.ByteStart:f.Index.ByteEnd])
	if tag != "#DDaraBot" {
		t.Errorf("facet tag text = %q, want %q", tag, "#DDaraBot")
	}
	if f.Features[0].Type != "app.bsky.richtext.facet#tag" {
		t.Errorf("feature type = %q", f.Features[0].Type)
	}
	if f.Features[0].Tag != "DDaraBot" {
		t.Errorf("feature tag = %q", f.Features[0].Tag)
	}
}

func TestPoster_PostReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(Session{
				AccessJwt: "token", RefreshJwt: "refresh", DID: "did:plc:me",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			if r.Header.Get("Authorization") != "Bearer token" {
				t.Error("missing auth header")
			}
			var req CreateRecordRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Repo != "did:plc:me" {
				t.Errorf("repo = %q, want %q", req.Repo, "did:plc:me")
			}
			json.NewEncoder(w).Encode(CreateRecordResponse{
				URI: "at://did:plc:me/app.bsky.feed.post/reply123",
				CID: "bafytest",
			})
		}
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "password")
	poster := NewPoster(auth, server.URL, slog.Default(), false)

	original := OriginalPost{
		URI: "at://did:plc:author/app.bsky.feed.post/orig123",
		CID: "bafyorig",
	}

	err := poster.PostReply(context.Background(), original, "en",
		"Hello world\n\n🌐 Translated by #DDaraBot")
	if err != nil {
		t.Fatalf("PostReply() error = %v", err)
	}
}

func TestPoster_ExpiredToken_Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(Session{
				AccessJwt: "token-new", RefreshJwt: "refresh", DID: "did:plc:me",
			})
		case "/xrpc/com.atproto.server.refreshSession":
			// Return error so it falls through to createSession
			w.WriteHeader(http.StatusUnauthorized)
		case "/xrpc/com.atproto.repo.createRecord":
			callCount++
			if callCount == 1 {
				// First call: return expired token error
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(XRPCError{
					Error:   "ExpiredToken",
					Message: "Token has expired",
				})
				return
			}
			// Second call: return success
			json.NewEncoder(w).Encode(CreateRecordResponse{
				URI: "at://did:plc:me/app.bsky.feed.post/reply456",
				CID: "bafyretry",
			})
		}
	}))
	defer server.Close()

	auth := NewAuth(server.URL, "test.bsky.social", "password")
	poster := NewPoster(auth, server.URL, slog.Default(), false)

	original := OriginalPost{
		URI: "at://did:plc:author/app.bsky.feed.post/orig123",
		CID: "bafyorig",
	}

	err := poster.PostReply(context.Background(), original, "en",
		"Hello world\n\n🌐 Translated by #DDaraBot")
	if err != nil {
		t.Fatalf("PostReply() error = %v, want nil after retry", err)
	}

	if callCount != 2 {
		t.Errorf("createRecord called %d times, want 2", callCount)
	}
}

func TestIsExpiredTokenError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorCode  string
		want       bool
	}{
		{"401 with any error code", http.StatusUnauthorized, "SomeError", true},
		{"400 with ExpiredToken", http.StatusBadRequest, "ExpiredToken", true},
		{"400 with InvalidToken", http.StatusBadRequest, "InvalidToken", true},
		{"400 with SomeOtherError", http.StatusBadRequest, "SomeOtherError", false},
		{"200 with empty error", http.StatusOK, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExpiredTokenError(tt.statusCode, tt.errorCode)
			if got != tt.want {
				t.Errorf("isExpiredTokenError(%d, %q) = %v, want %v",
					tt.statusCode, tt.errorCode, got, tt.want)
			}
		})
	}
}

func TestPoster_DryRun(t *testing.T) {
	poster := NewPoster(nil, "", slog.Default(), true)

	err := poster.PostReply(context.Background(), OriginalPost{
		URI: "at://test/app.bsky.feed.post/123",
		CID: "bafytest",
	}, "en", "translated text")

	if err != nil {
		t.Fatalf("PostReply() dry-run error = %v", err)
	}
}
