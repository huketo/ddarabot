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
