package filter

import (
	"encoding/json"
	"testing"
)

func TestHasTriggerTag(t *testing.T) {
	tests := []struct {
		name   string
		facets json.RawMessage
		tag    string
		want   bool
	}{
		{
			name: "has ddara tag",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 20, "byteEnd": 26},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "ddara"}]
			}]`),
			tag:  "ddara",
			want: true,
		},
		{
			name: "case insensitive",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 0, "byteEnd": 6},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "DDara"}]
			}]`),
			tag:  "ddara",
			want: true,
		},
		{
			name: "no matching tag",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 0, "byteEnd": 5},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "other"}]
			}]`),
			tag:  "ddara",
			want: false,
		},
		{
			name:   "no facets",
			facets: nil,
			tag:    "ddara",
			want:   false,
		},
		{
			name:   "empty facets array",
			facets: json.RawMessage(`[]`),
			tag:    "ddara",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasTriggerTag(tt.facets, tt.tag)
			if got != tt.want {
				t.Errorf("HasTriggerTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveTriggerTag(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		facets json.RawMessage
		tag    string
		want   string
	}{
		{
			// Korean text before "#ddara" = 28 bytes (9 CJK chars x 3 bytes + "! " 2 bytes)
			// "#ddara" = 6 bytes, so byteStart=28, byteEnd=34
			// '#'(1)+'d'(1)+'d'(1)+'a'(1)+'r'(1)+'a'(1) = 6, total = 34
			name: "remove trailing #ddara",
			text: "오늘 날씨가 좋네요! #ddara",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 28, "byteEnd": 34},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "ddara"}]
			}]`),
			tag:  "ddara",
			want: "오늘 날씨가 좋네요!",
		},
		{
			name: "remove #ddara with leading space",
			text: "Hello world #ddara",
			facets: json.RawMessage(`[{
				"index": {"byteStart": 12, "byteEnd": 18},
				"features": [{"$type": "app.bsky.richtext.facet#tag", "tag": "ddara"}]
			}]`),
			tag:  "ddara",
			want: "Hello world",
		},
		{
			name:   "no ddara tag to remove",
			text:   "Hello world",
			facets: nil,
			tag:    "ddara",
			want:   "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveTriggerTag(tt.text, tt.facets, tt.tag)
			if got != tt.want {
				t.Errorf("RemoveTriggerTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsReply(t *testing.T) {
	tests := []struct {
		name   string
		record json.RawMessage
		want   bool
	}{
		{
			name:   "is a reply",
			record: json.RawMessage(`{"$type":"app.bsky.feed.post","text":"reply","reply":{"root":{},"parent":{}}}`),
			want:   true,
		},
		{
			name:   "not a reply",
			record: json.RawMessage(`{"$type":"app.bsky.feed.post","text":"original post"}`),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsReply(tt.record)
			if got != tt.want {
				t.Errorf("IsReply() = %v, want %v", got, tt.want)
			}
		})
	}
}
