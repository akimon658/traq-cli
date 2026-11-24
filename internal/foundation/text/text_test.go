package text

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	markdown "github.com/traPtitech/traq-flavored-markdown/packages/sdk/go"
)

// stubProcessor は process 呼び出しを記録し、差し込んだ関数に委譲する。
type stubProcessor struct {
	calls     []string
	onProcess func(ctx context.Context, source string) (*markdown.ProcessedDocument, error)
}

func (s *stubProcessor) Process(
	ctx context.Context,
	source string,
) (*markdown.ProcessedDocument, error) {
	s.calls = append(s.calls, source)
	return s.onProcess(ctx, source)
}

func mustDecode(t *testing.T, raw string) *markdown.Document {
	t.Helper()
	doc, err := markdown.DecodeDocument([]byte(raw))
	require.NoError(t, err)
	return doc
}

// fixtureHello は "hello" を traq.v1 で処理した Document の wire JSON。
const fixtureHello = `{"source":"hello","children":[{"kind":"commonmark.paragraph","span":{"start":0,"end":5},"data":{},"children":[{"kind":"commonmark.text","span":{"start":0,"end":5},"data":{"value":"hello"}}]}]}`

// fixtureRichSource は fixtureRich に対応する入力ソース。
const fixtureRichSource = `hi !{"type":"user","raw":"@alice","id":"dfdff0c9-5de0-46ee-9721-2525e8bb3d44"} and !{"type":"group","raw":"@team","id":"dfabf0c9-5de0-46ee-9721-2525e8bb3d45"}` +
	"\n" + `!{"type":"message","raw":"q","id":"11111111-1111-1111-1111-111111111111"}` +
	` !{"type":"file","raw":"a.png","id":"33333333-3333-3333-3333-333333333333"}`

// fixtureRich は fixtureRichSource を traq.v1 で処理した Document の wire JSON (v0.3.0)。
const fixtureRich = `{"source":"hi !{\"type\":\"user\",\"raw\":\"@alice\",\"id\":\"dfdff0c9-5de0-46ee-9721-2525e8bb3d44\"} and !{\"type\":\"group\",\"raw\":\"@team\",\"id\":\"dfabf0c9-5de0-46ee-9721-2525e8bb3d45\"}\n!{\"type\":\"message\",\"raw\":\"q\",\"id\":\"11111111-1111-1111-1111-111111111111\"} !{\"type\":\"file\",\"raw\":\"a.png\",\"id\":\"33333333-3333-3333-3333-333333333333\"}","children":[{"kind":"commonmark.paragraph","span":{"start":0,"end":307},"data":{},"children":[{"kind":"commonmark.text","span":{"start":0,"end":3},"data":{"value":"hi "}},{"kind":"traq.reference","span":{"start":3,"end":78},"data":{"type":"user","id":"dfdff0c9-5de0-46ee-9721-2525e8bb3d44","label":"@alice"}},{"kind":"commonmark.text","span":{"start":78,"end":83},"data":{"value":" and "}},{"kind":"traq.reference","span":{"start":83,"end":158},"data":{"type":"group","id":"dfabf0c9-5de0-46ee-9721-2525e8bb3d45","label":"@team"}},{"kind":"commonmark.softbreak","span":{"start":158,"end":159},"data":{}},{"kind":"traq.embedding","span":{"start":159,"end":232},"data":{"type":"message","id":"11111111-1111-1111-1111-111111111111","label":"q","literal":"!{\"type\":\"message\",\"raw\":\"q\",\"id\":\"11111111-1111-1111-1111-111111111111\"}"}},{"kind":"commonmark.text","span":{"start":232,"end":233},"data":{"value":" "}},{"kind":"traq.embedding","span":{"start":233,"end":307},"data":{"type":"file","id":"33333333-3333-3333-3333-333333333333","label":"a.png","literal":"!{\"type\":\"file\",\"raw\":\"a.png\",\"id\":\"33333333-3333-3333-3333-333333333333\"}"}}]}]}`

const (
	fixtureUserID  = "dfdff0c9-5de0-46ee-9721-2525e8bb3d44"
	fixtureGroupID = "dfabf0c9-5de0-46ee-9721-2525e8bb3d45"
	fixtureQuoteID = "11111111-1111-1111-1111-111111111111"
	fixtureFileID  = "33333333-3333-3333-3333-333333333333"
)

func TestParseWith(t *testing.T) {
	t.Parallel()

	t.Run("plain content single pass", func(t *testing.T) {
		t.Parallel()
		stub := &stubProcessor{
			onProcess: func(_ context.Context, _ string) (*markdown.ProcessedDocument, error) {
				return &markdown.ProcessedDocument{
					Document: mustDecode(t, fixtureHello),
					Extraction: markdown.Extraction{
						MessageText: "hello",
						Citations:   []string{fixtureQuoteID},
						Attachments: []string{fixtureFileID},
					},
				}, nil
			},
		}
		got, err := parseWith(t.Context(), stub, ParseInput{Content: "hello"}, true)
		require.NoError(t, err)
		assert.Equal(t, "hello", got.Body)
		assert.Equal(t, []string{fixtureQuoteID}, got.QuoteIDs)
		assert.Equal(t, []string{fixtureFileID}, got.FileIDs)
		assert.False(t, got.SelfMentioned)
		assert.Equal(t, []string{"hello"}, stub.calls)
	})

	t.Run("highlight and embed strip second pass", func(t *testing.T) {
		t.Parallel()
		stub := &stubProcessor{
			onProcess: func(_ context.Context, source string) (*markdown.ProcessedDocument, error) {
				if source == fixtureRichSource {
					return &markdown.ProcessedDocument{
						Document: mustDecode(t, fixtureRich),
						Extraction: markdown.Extraction{
							MessageText: "first",
							Citations:   []string{fixtureQuoteID},
							Attachments: []string{fixtureFileID},
						},
					}, nil
				}
				return &markdown.ProcessedDocument{
					Extraction: markdown.Extraction{MessageText: "second"},
				}, nil
			},
		}
		in := ParseInput{
			Content: fixtureRichSource,
			SelfID:  fixtureUserID,
			Groups:  map[string]struct{}{fixtureGroupID: {}},
		}
		got, err := parseWith(t.Context(), stub, in, true)
		require.NoError(t, err)
		assert.Equal(t, "second", got.Body)
		assert.Equal(t, []string{fixtureQuoteID}, got.QuoteIDs)
		assert.Equal(t, []string{fixtureFileID}, got.FileIDs)
		assert.True(t, got.SelfMentioned)
		require.Len(t, stub.calls, 2)
		rewritten := stub.calls[1]
		assert.Contains(t, rewritten, `"raw":"@alice [to you]"`)
		assert.Contains(t, rewritten, `"raw":"@team [to you]"`)
		assert.NotContains(t, rewritten, `"type":"message"`)
		assert.NotContains(t, rewritten, `"type":"file"`)
	})

	t.Run("quote keeps embed labels", func(t *testing.T) {
		t.Parallel()
		stub := &stubProcessor{
			onProcess: func(_ context.Context, _ string) (*markdown.ProcessedDocument, error) {
				return &markdown.ProcessedDocument{
					Document: mustDecode(t, fixtureRich),
					Extraction: markdown.Extraction{
						MessageText: "first",
						Citations:   []string{fixtureQuoteID},
						Attachments: []string{fixtureFileID},
					},
				}, nil
			},
		}
		// 自分宛なし・除去なしなら書き換え不要で1回で終わる。
		got, err := parseWith(t.Context(), stub, ParseInput{Content: fixtureRichSource}, false)
		require.NoError(t, err)
		assert.Equal(t, "first", got.Body)
		assert.Equal(t, []string{fixtureQuoteID}, got.QuoteIDs)
		assert.Equal(t, []string{fixtureFileID}, got.FileIDs)
		assert.False(t, got.SelfMentioned)
		assert.Len(t, stub.calls, 1)
	})

	t.Run("first pass error", func(t *testing.T) {
		t.Parallel()
		stub := &stubProcessor{
			onProcess: func(_ context.Context, _ string) (*markdown.ProcessedDocument, error) {
				return nil, assert.AnError
			},
		}
		_, err := parseWith(t.Context(), stub, ParseInput{Content: "hello"}, true)
		require.Error(t, err)
	})

	t.Run("second pass error", func(t *testing.T) {
		t.Parallel()
		calls := 0
		stub := &stubProcessor{
			onProcess: func(_ context.Context, _ string) (*markdown.ProcessedDocument, error) {
				calls++
				if calls == 1 {
					return &markdown.ProcessedDocument{
						Document:   mustDecode(t, fixtureRich),
						Extraction: markdown.Extraction{MessageText: "first"},
					}, nil
				}
				return nil, assert.AnError
			},
		}
		in := ParseInput{Content: fixtureRichSource, SelfID: fixtureUserID}
		_, err := parseWith(t.Context(), stub, in, true)
		require.Error(t, err)
	})
}
