package bot

import (
	"encoding/json"
	"testing"

	"github.com/huketo/ddarabot/internal/jetstream"
)

func TestExtractLinkInfos(t *testing.T) {
	post := jetstream.Post{
		Text: "Visit example.com for more",
		Facets: json.RawMessage(`[{
			"index": {"byteStart": 6, "byteEnd": 17},
			"features": [{"$type": "app.bsky.richtext.facet#link", "uri": "https://example.com"}]
		}]`),
	}

	links := extractLinkInfos(post)
	if len(links) != 1 {
		t.Fatalf("extractLinkInfos() returned %d links, want 1", len(links))
	}
	if links[0].DisplayText != "example.com" {
		t.Errorf("DisplayText = %q, want %q", links[0].DisplayText, "example.com")
	}
	if links[0].URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", links[0].URL, "https://example.com")
	}
}

func TestExtractEmbed(t *testing.T) {
	embedJSON := `{"$type":"app.bsky.embed.external","external":{"uri":"https://example.com","title":"Example","description":"Desc"}}`
	record := json.RawMessage(`{"$type":"app.bsky.feed.post","text":"hello","embed":` + embedJSON + `}`)

	post := jetstream.Post{
		Record: record,
	}

	got := extractEmbed(post)
	if got == nil {
		t.Fatal("extractEmbed() returned nil, expected embed JSON")
	}

	// Verify the extracted embed matches
	var embed map[string]interface{}
	if err := json.Unmarshal(got, &embed); err != nil {
		t.Fatalf("unmarshal embed: %v", err)
	}
	if embed["$type"] != "app.bsky.embed.external" {
		t.Errorf("embed $type = %v, want app.bsky.embed.external", embed["$type"])
	}
}

func TestExtractEmbed_NoEmbed(t *testing.T) {
	record := json.RawMessage(`{"$type":"app.bsky.feed.post","text":"hello"}`)
	post := jetstream.Post{
		Record: record,
	}

	got := extractEmbed(post)
	if got != nil {
		t.Errorf("extractEmbed() = %s, want nil", string(got))
	}
}

func TestPostURI(t *testing.T) {
	uri := buildPostURI("did:plc:test", "abc123")
	want := "at://did:plc:test/app.bsky.feed.post/abc123"
	if uri != want {
		t.Errorf("buildPostURI() = %q, want %q", uri, want)
	}
}
