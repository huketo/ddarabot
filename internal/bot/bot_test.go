package bot

import (
	"testing"
)

func TestPostURI(t *testing.T) {
	uri := buildPostURI("did:plc:test", "abc123")
	want := "at://did:plc:test/app.bsky.feed.post/abc123"
	if uri != want {
		t.Errorf("buildPostURI() = %q, want %q", uri, want)
	}
}
