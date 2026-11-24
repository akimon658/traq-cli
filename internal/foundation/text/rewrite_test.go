package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func userNode(start, end uint32, id string) nodeRef {
	return nodeRef{kind: refUser, start: start, end: end, id: id}
}

func TestCollectNodes(t *testing.T) {
	t.Parallel()

	t.Run("rich fixture", func(t *testing.T) {
		t.Parallel()
		got, err := collectNodes(mustDecode(t, fixtureRich))
		require.NoError(t, err)
		require.Len(t, got, 4)
		assert.Equal(t, nodeRef{kind: refUser, start: 3, end: 78, id: fixtureUserID}, got[0])
		assert.Equal(t, nodeRef{kind: refGroup, start: 83, end: 158, id: fixtureGroupID}, got[1])
		assert.Equal(t, nodeRef{kind: embMessage, start: 159, end: 232, id: fixtureQuoteID}, got[2])
		assert.Equal(t, nodeRef{kind: embFile, start: 233, end: 307, id: fixtureFileID}, got[3])
	})

	t.Run("plain text has no nodes", func(t *testing.T) {
		t.Parallel()
		got, err := collectNodes(mustDecode(t, fixtureHello))
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("invalid document", func(t *testing.T) {
		t.Parallel()
		_, err := collectNodes(nil)
		require.Error(t, err)
	})
}

func TestMarkSelfMention(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		got, err := markSelfMention(`!{"type":"user","raw":"@alice","id":"u1"}`)
		require.NoError(t, err)
		assert.Contains(t, got, `"raw":"@alice [to you]"`)
		assert.Contains(t, got, `"id":"u1"`)
		assert.Contains(t, got, `"type":"user"`)
	})

	t.Run("broken json", func(t *testing.T) {
		t.Parallel()
		_, err := markSelfMention(`!{"type":"user","raw":"@alice"`)
		require.Error(t, err)
	})
}

func TestRewriterRewrite(t *testing.T) {
	t.Parallel()

	const (
		userEmbed = `!{"type":"user","raw":"@alice","id":"u1"}`
		fileEmbed = `!{"type":"file","raw":"a.png","id":"f1"}`
		msgEmbed  = `!{"type":"message","raw":"q","id":"m1"}`
	)

	t.Run("direct mention", func(t *testing.T) {
		t.Parallel()
		bob := `!{"type":"user","raw":"@bob","id":"u2"}`
		source := "hi " + userEmbed + " and " + bob
		nodes := []nodeRef{userNode(3, 3+uint32(len(userEmbed)), "u1")}
		w := rewriter{selfID: "u1", stripEmbeds: true}
		got, mentioned, err := w.rewrite(source, nodes)
		require.NoError(t, err)
		assert.True(t, mentioned)
		assert.Contains(t, got, "@alice [to you]")
		assert.NotContains(t, got, "@bob [to you]")
	})

	t.Run("case insensitive id", func(t *testing.T) {
		t.Parallel()
		source := "hi " + userEmbed
		s, e := uint32(3), uint32(3+len(userEmbed))
		w := rewriter{selfID: "U1", stripEmbeds: false}
		got, mentioned, err := w.rewrite(source, []nodeRef{userNode(s, e, "u1")})
		require.NoError(t, err)
		assert.True(t, mentioned)
		assert.Contains(t, got, "@alice [to you]")
	})

	t.Run("empty self id no marker", func(t *testing.T) {
		t.Parallel()
		source := "hi " + userEmbed
		s, e := uint32(3), uint32(3+len(userEmbed))
		w := rewriter{stripEmbeds: false}
		got, mentioned, err := w.rewrite(source, []nodeRef{userNode(s, e, "u1")})
		require.NoError(t, err)
		assert.False(t, mentioned)
		assert.Equal(t, source, got)
	})

	t.Run("group mention", func(t *testing.T) {
		t.Parallel()
		team := `!{"type":"group","raw":"@team","id":"g1"}`
		other := `!{"type":"group","raw":"@other","id":"g2"}`
		source := "hi " + team + " and " + other
		s1, e1 := uint32(3), uint32(3+len(team))
		s2, e2 := uint32(3+len(team)+len(" and ")), uint32(3+len(team)+len(" and ")+len(other))
		nodes := []nodeRef{
			{kind: refGroup, start: s1, end: e1, id: "g1"},
			{kind: refGroup, start: s2, end: e2, id: "g2"},
		}
		w := rewriter{selfID: "u9", groups: map[string]struct{}{"G1": {}}, stripEmbeds: false}
		// グループ照合は正規化済みを想定するため、ここでは正規化して渡す。
		w.groups = normalizeGroups(w.groups)
		got, mentioned, err := w.rewrite(source, nodes)
		require.NoError(t, err)
		assert.True(t, mentioned)
		assert.Contains(t, got, "@team [to you]")
		assert.NotContains(t, got, "@other [to you]")
	})

	t.Run("channel untouched", func(t *testing.T) {
		t.Parallel()
		ch := `!{"type":"channel","raw":"#general","id":"c1"}`
		source := "hi " + ch
		s, e := uint32(3), uint32(3+len(ch))
		nodes := []nodeRef{{kind: refChannel, start: s, end: e, id: "c1"}}
		w := rewriter{selfID: "c1", stripEmbeds: true}
		got, mentioned, err := w.rewrite(source, nodes)
		require.NoError(t, err)
		assert.False(t, mentioned)
		assert.Equal(t, source, got)
	})

	t.Run("bare reference skipped", func(t *testing.T) {
		t.Parallel()
		source := "hi @alice"
		nodes := []nodeRef{userNode(3, 9, "u1")}
		w := rewriter{selfID: "u1", stripEmbeds: false}
		got, mentioned, err := w.rewrite(source, nodes)
		require.NoError(t, err)
		assert.False(t, mentioned)
		assert.Equal(t, source, got)
	})

	t.Run("strip embeds", func(t *testing.T) {
		t.Parallel()
		source := "see " + msgEmbed + " " + fileEmbed + " end"
		s1 := uint32(len("see "))
		msgStart := s1 + uint32(len(msgEmbed)) + 1
		nodes := []nodeRef{
			{kind: embMessage, start: s1, end: s1 + uint32(len(msgEmbed))},
			{kind: embFile, start: msgStart, end: msgStart + uint32(len(fileEmbed))},
		}
		w := rewriter{stripEmbeds: true}
		got, mentioned, err := w.rewrite(source, nodes)
		require.NoError(t, err)
		assert.False(t, mentioned)
		assert.Equal(t, "see   end", got)
	})

	t.Run("keep embeds for quote", func(t *testing.T) {
		t.Parallel()
		source := "see " + msgEmbed + " end"
		s := uint32(len("see "))
		nodes := []nodeRef{{kind: embMessage, start: s, end: s + uint32(len(msgEmbed))}}
		w := rewriter{stripEmbeds: false}
		got, mentioned, err := w.rewrite(source, nodes)
		require.NoError(t, err)
		assert.False(t, mentioned)
		assert.Equal(t, source, got)
	})

	t.Run("span out of range", func(t *testing.T) {
		t.Parallel()
		w := rewriter{stripEmbeds: true}
		_, _, err := w.rewrite("hi", []nodeRef{{kind: embFile, start: 3, end: 100}})
		require.Error(t, err)
	})
}

func TestNormalizeGroups(t *testing.T) {
	t.Parallel()

	assert.Nil(t, normalizeGroups(nil))
	assert.Equal(
		t,
		map[string]struct{}{"g1": {}},
		normalizeGroups(map[string]struct{}{"G1": {}}),
	)
}
