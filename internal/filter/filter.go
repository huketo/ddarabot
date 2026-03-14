package filter

import (
	"encoding/json"
	"strings"
)

type facet struct {
	Index    facetIndex     `json:"index"`
	Features []facetFeature `json:"features"`
}

type facetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type facetFeature struct {
	Type string `json:"$type"`
	Tag  string `json:"tag"`
}

type replyCheck struct {
	Reply *json.RawMessage `json:"reply"`
}

func HasTriggerTag(facetsJSON json.RawMessage, triggerTag string) bool {
	if len(facetsJSON) == 0 {
		return false
	}
	var facets []facet
	if err := json.Unmarshal(facetsJSON, &facets); err != nil {
		return false
	}
	for _, f := range facets {
		for _, feat := range f.Features {
			if feat.Type == "app.bsky.richtext.facet#tag" &&
				strings.EqualFold(feat.Tag, triggerTag) {
				return true
			}
		}
	}
	return false
}

func RemoveTriggerTag(text string, facetsJSON json.RawMessage, triggerTag string) string {
	if len(facetsJSON) == 0 {
		return text
	}
	var facets []facet
	if err := json.Unmarshal(facetsJSON, &facets); err != nil {
		return text
	}

	textBytes := []byte(text)
	for _, f := range facets {
		for _, feat := range f.Features {
			if feat.Type == "app.bsky.richtext.facet#tag" &&
				strings.EqualFold(feat.Tag, triggerTag) {
				start := f.Index.ByteStart
				end := f.Index.ByteEnd
				if start > len(textBytes) || end > len(textBytes) {
					continue
				}
				if start > 0 && textBytes[start-1] == ' ' {
					start--
				}
				result := make([]byte, 0, len(textBytes))
				result = append(result, textBytes[:start]...)
				result = append(result, textBytes[end:]...)
				return strings.TrimSpace(string(result))
			}
		}
	}
	return text
}

func IsReply(record json.RawMessage) bool {
	var rc replyCheck
	if err := json.Unmarshal(record, &rc); err != nil {
		return false
	}
	return rc.Reply != nil
}
