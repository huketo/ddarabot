package bluesky

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/huketo/ddarabot/internal/filter"
)

type OriginalPost struct {
	URI       string
	CID       string
	Embed     json.RawMessage
	LinkInfos []filter.LinkInfo
}

type Poster struct {
	auth    *Auth
	pdsHost string
	logger  *slog.Logger
	dryRun  bool
	client  *http.Client
}

func NewPoster(auth *Auth, pdsHost string, logger *slog.Logger, dryRun bool) *Poster {
	return &Poster{
		auth:    auth,
		pdsHost: pdsHost,
		logger:  logger,
		dryRun:  dryRun,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Poster) PostReply(ctx context.Context, original OriginalPost, lang, text string) error {
	if p.dryRun {
		p.logger.Info("[dry-run] would post reply", "lang", lang, "text", text, "parent", original.URI)
		return nil
	}
	return p.postReplyWithRetry(ctx, original, lang, text, false)
}

func (p *Poster) postReplyWithRetry(ctx context.Context, original OriginalPost, lang, text string, retried bool) error {
	session, err := p.auth.GetSession(ctx)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	facets := BuildAllHashtagFacets(text)
	facets = append(facets, BuildLinkFacets(text, original.LinkInfos)...)

	var embed *json.RawMessage
	if len(original.Embed) > 0 {
		embed = &original.Embed
	}

	record := PostRecord{
		Type:      "app.bsky.feed.post",
		Text:      text,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Langs:     []string{lang},
		Reply: &ReplyRef{
			Root:   StrongRef{URI: original.URI, CID: original.CID},
			Parent: StrongRef{URI: original.URI, CID: original.CID},
		},
		Facets: facets,
		Embed:  embed,
	}

	reqBody := CreateRecordRequest{
		Repo:       session.DID,
		Collection: "app.bsky.feed.post",
		Record:     record,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.pdsHost+"/xrpc/com.atproto.repo.createRecord",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.AccessJwt)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("create record: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var xrpcErr XRPCError
		json.NewDecoder(resp.Body).Decode(&xrpcErr)

		if !retried && isExpiredTokenError(resp.StatusCode, xrpcErr.Error) {
			p.auth.InvalidateSession()
			p.logger.Warn("session expired, re-authenticating", "lang", lang)
			return p.postReplyWithRetry(ctx, original, lang, text, true)
		}

		return fmt.Errorf("create record: %d %s %s", resp.StatusCode, xrpcErr.Error, xrpcErr.Message)
	}

	var result CreateRecordResponse
	json.NewDecoder(resp.Body).Decode(&result)
	p.logger.Info("posted reply", "lang", lang, "uri", result.URI)
	return nil
}

// PostError wraps a posting error with the language that failed.
type PostError struct {
	Lang string
	Err  error
}

func (e *PostError) Error() string {
	return fmt.Sprintf("post %s: %s", e.Lang, e.Err)
}

func (e *PostError) Unwrap() error {
	return e.Err
}

func (p *Poster) PostAll(ctx context.Context, original OriginalPost, translations map[string]string, maxConcurrent int) []error {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	type result struct {
		lang string
		err  error
	}

	sem := make(chan struct{}, maxConcurrent)
	ch := make(chan result, len(translations))

	for lang, text := range translations {
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			ch <- result{lang: lang, err: p.PostReply(ctx, original, lang, text)}
		}()
	}

	var errs []error
	for range translations {
		r := <-ch
		if r.err != nil {
			errs = append(errs, &PostError{Lang: r.lang, Err: r.err})
		}
	}
	return errs
}

func isExpiredTokenError(statusCode int, errorCode string) bool {
	return statusCode == http.StatusUnauthorized ||
		errorCode == "ExpiredToken" ||
		errorCode == "InvalidToken"
}

// BuildLinkFacets finds link display texts in the translated text and creates link facets.
func BuildLinkFacets(text string, links []filter.LinkInfo) []PostFacet {
	var facets []PostFacet
	for _, link := range links {
		idx := strings.Index(text, link.DisplayText)
		if idx == -1 {
			continue
		}
		byteStart := idx
		byteEnd := byteStart + len(link.DisplayText)
		facets = append(facets, PostFacet{
			Index: FacetIndex{ByteStart: byteStart, ByteEnd: byteEnd},
			Features: []FacetFeature{
				{
					Type: "app.bsky.richtext.facet#link",
					URI:  link.URL,
				},
			},
		})
	}
	return facets
}

var hashtagRe = regexp.MustCompile(`#([\p{L}\p{N}_]+)`)

// BuildAllHashtagFacets finds all #hashtag patterns in the text and creates tag facets.
func BuildAllHashtagFacets(text string) []PostFacet {
	matches := hashtagRe.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	var facets []PostFacet
	for _, m := range matches {
		tag := text[m[0]+1 : m[1]] // skip the '#'
		facets = append(facets, PostFacet{
			Index: FacetIndex{ByteStart: m[0], ByteEnd: m[1]},
			Features: []FacetFeature{
				{
					Type: "app.bsky.richtext.facet#tag",
					Tag:  tag,
				},
			},
		})
	}
	return facets
}
