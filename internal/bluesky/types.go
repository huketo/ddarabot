package bluesky

import "encoding/json"

type CreateSessionRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type Session struct {
	AccessJwt  string `json:"accessJwt"`
	RefreshJwt string `json:"refreshJwt"`
	DID        string `json:"did"`
	Handle     string `json:"handle"`
}

type CreateRecordRequest struct {
	Repo       string      `json:"repo"`
	Collection string      `json:"collection"`
	Record     interface{} `json:"record"`
}

type CreateRecordResponse struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type PostRecord struct {
	Type      string           `json:"$type"`
	Text      string           `json:"text"`
	CreatedAt string           `json:"createdAt"`
	Langs     []string         `json:"langs,omitempty"`
	Reply     *ReplyRef        `json:"reply,omitempty"`
	Facets    []PostFacet      `json:"facets,omitempty"`
	Embed     *json.RawMessage `json:"embed,omitempty"`
}

type ReplyRef struct {
	Root   StrongRef `json:"root"`
	Parent StrongRef `json:"parent"`
}

type StrongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type PostFacet struct {
	Index    FacetIndex     `json:"index"`
	Features []FacetFeature `json:"features"`
}

type FacetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type FacetFeature struct {
	Type string `json:"$type"`
	Tag  string `json:"tag,omitempty"`
	URI  string `json:"uri,omitempty"`
}

type XRPCError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
