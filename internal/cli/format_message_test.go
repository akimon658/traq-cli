package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/moznion/go-optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/traPtitech/go-traq"
)

func TestWriteMessageListOrderAndFormat(t *testing.T) {
	t.Parallel()
	loc := time.UTC
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	users := map[string]traq.User{
		"u1": {Id: "u1", Name: "alice", DisplayName: "Alice Smith"},
		"u2": {Id: "u2", Name: "bob", DisplayName: "bob"},
	}
	// 取得順は desc (新しい順) を想定
	msgs := []traq.Message{
		{
			Id:        "m2",
			UserId:    "u2",
			Content:   "newer\nlong line",
			CreatedAt: base.Add(time.Minute),
			UpdatedAt: base.Add(2 * time.Minute),
			Pinned:    true,
		},
		{Id: "m1", UserId: "u1", Content: "older", CreatedAt: base, UpdatedAt: base},
	}

	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "#general", limit: 10, offset: 0, order: "desc",
		msgs: msgs, users: users, loc: loc,
		stamps: map[string]string{},
	}))

	want := strings.Join([]string{
		"#general (2件 limit=10 offset=0 order=desc)",
		"",
		"2026-10-17T10:00:00Z @alice (Alice Smith) id:m1",
		"  older",
		"",
		"2026-10-17T10:01:00Z @bob id:m2 [edited] [pinned]",
		"  newer",
		"  long line",
		"",
	}, "\n")
	assert.Equal(t, want, buf.String())
}

func TestWriteMessageListStamps(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	msgs := []traq.Message{{
		Id: "m1", UserId: "u1", Content: "hello",
		CreatedAt: base, UpdatedAt: base,
		Stamps: []traq.MessageStamp{
			{StampId: "s2", Count: 3},
			{StampId: "s1", Count: 1},
		},
	}}

	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "#general", limit: 10, order: "desc",
		msgs: msgs,
		users: map[string]traq.User{
			"u1": {Id: "u1", Name: "alice", DisplayName: "alice"},
		},
		stamps: map[string]string{"s1": "ok", "s2": "thank"},
		loc:    time.UTC,
	}))

	want := strings.Join([]string{
		"#general (1件 limit=10 offset=0 order=desc)",
		"",
		"2026-10-17T10:00:00Z @alice id:m1",
		"  hello",
		"スタンプ",
		"  :thank: ×3",
		"  :ok: ×1",
		"",
	}, "\n")
	assert.Equal(t, want, buf.String())
}

func TestFormatStampItems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stamps []traq.MessageStamp
		names  map[string]string
		want   []string
	}{
		{
			name:   "empty",
			stamps: nil,
			names:  map[string]string{},
			want:   nil,
		},
		{
			name: "preserves API order",
			stamps: []traq.MessageStamp{
				{StampId: "s2", Count: 3},
				{StampId: "s1", Count: 1},
			},
			names: map[string]string{"s1": "ok", "s2": "thank"},
			want:  []string{":thank: ×3", ":ok: ×1"},
		},
		{
			name: "unknown stamp falls back to ID",
			stamps: []traq.MessageStamp{
				{StampId: "s9", Count: 2},
			},
			names: map[string]string{},
			want:  []string{":s9: ×2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatStampItems(tt.stamps, tt.names))
		})
	}
}

func TestWriteMessageListUnreadDivider(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	users := map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}}
	msgs := []traq.Message{
		{Id: "m1", UserId: "u1", Content: "read", CreatedAt: base, UpdatedAt: base},
		{
			Id:        "m2",
			UserId:    "u1",
			Content:   "unread",
			CreatedAt: base.Add(time.Minute),
			UpdatedAt: base.Add(time.Minute),
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label:  "#general",
		limit:  10,
		order:  "desc",
		msgs:   msgs,
		users:  users,
		stamps: map[string]string{},
		loc:    time.UTC,
		unread: optional.Some(traq.UnreadChannel{
			ChannelId:       "c1",
			Count:           5,
			Noticeable:      true,
			OldestMessageId: "m2",
		}),
	}))

	out := buf.String()
	dividerIdx := strings.Index(out, "----- ここから未読 (5件) [mentionあり] -----")
	m2Idx := strings.Index(out, "id:m2")
	require.NotEqual(t, -1, dividerIdx, "divider not found: %s", out)
	require.NotEqual(t, -1, m2Idx, "m2 not found: %s", out)
	assert.Less(t, dividerIdx, m2Idx, "divider should come before the first unread message")
	assert.Contains(t, out, "id:m1")
}

func TestWriteMessageListUnreadNotInPage(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "#general",
		limit: 10,
		order: "desc",
		msgs: []traq.Message{
			{Id: "m1", UserId: "u1", Content: "hi", CreatedAt: base, UpdatedAt: base},
		},
		users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		stamps: map[string]string{},
		loc:    time.UTC,
		unread: optional.Some(
			traq.UnreadChannel{ChannelId: "c1", Count: 3, OldestMessageId: "other"},
		),
	}))
	assert.Contains(t, buf.String(), "# 未読 3 件 (このページに未読の先頭は含まれない)")
}

func TestWriteMessageListUnreadOnly(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	users := map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}}
	msgs := []traq.Message{
		{Id: "m1", UserId: "u1", Content: "unread-1", CreatedAt: base, UpdatedAt: base},
		{
			Id:        "m2",
			UserId:    "u1",
			Content:   "unread-2",
			CreatedAt: base.Add(time.Minute),
			UpdatedAt: base.Add(time.Minute),
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label:  "#general",
		limit:  10,
		order:  "desc",
		msgs:   msgs,
		users:  users,
		stamps: map[string]string{},
		loc:    time.UTC,
		unread: optional.Some(traq.UnreadChannel{
			ChannelId:       "c1",
			Count:           7,
			Noticeable:      true,
			OldestMessageId: "m1",
		}),
		unreadOnly: true,
	}))

	out := buf.String()
	assert.Contains(t, out, "#general (未読7件中 2件 limit=10 offset=0 order=desc)")
	assert.NotContains(t, out, "ここから未読", "the divider should be suppressed in unread-only mode")
	assert.NotContains(t, out, "このページに未読の先頭は含まれない")
	i1 := strings.Index(out, "id:m1")
	i2 := strings.Index(out, "id:m2")
	require.NotEqual(t, -1, i1)
	require.NotEqual(t, -1, i2)
	assert.Less(t, i1, i2, "messages should be displayed oldest first")
}

func TestWriteMessageListUnreadOnlyPartial(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "#general",
		limit: 2,
		order: "desc",
		msgs: []traq.Message{
			{Id: "m1", UserId: "u1", Content: "hi", CreatedAt: base, UpdatedAt: base},
		},
		users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		stamps: map[string]string{},
		loc:    time.UTC,
		unread: optional.Some(traq.UnreadChannel{
			ChannelId:       "c1",
			Count:           5,
			OldestMessageId: "m9",
		}),
		unreadOnly: true,
	}))
	assert.Contains(t, buf.String(), "#general (未読5件中 1件 limit=2 offset=0 order=desc)")
}

func TestWriteMessageListUnreadOnlyNoUnread(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label:      "#general",
		limit:      10,
		order:      "desc",
		msgs:       nil,
		users:      map[string]traq.User{},
		stamps:     map[string]string{},
		loc:        time.UTC,
		unreadOnly: true,
	}))
	assert.Equal(t, "#general (未読0件)\n\n", buf.String())
}

func TestWriteMessageListUserNotFoundFallback(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	err := writeMessageList(&buf, messageList{
		label: "#general", limit: 10, order: "desc",
		msgs:   []traq.Message{{Id: "m1", UserId: "missing", CreatedAt: base, UpdatedAt: base}},
		users:  map[string]traq.User{},
		stamps: map[string]string{}, loc: time.UTC,
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "(ユーザー解決失敗 id:missing)")
	assert.NotContains(
		t,
		buf.String(),
		"@unknown",
		"fallback must not collide with a real user name",
	)
}

func TestFormatQuoteUserNotFoundFallback(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	ml := messageList{
		users: map[string]traq.User{},
		loc:   time.UTC,
		quoted: map[string]traq.Message{
			"q1": {
				Id:        "q1",
				UserId:    "gone",
				Content:   "quoted text",
				CreatedAt: base,
				UpdatedAt: base,
			},
		},
	}
	lines := formatQuote("q1", ml)
	out := strings.Join(lines, "\n")
	assert.Contains(t, out, "(ユーザー解決失敗 id:gone)")
	assert.NotContains(
		t,
		out,
		"@unknown",
		"fallback must not collide with a real user name",
	)
	assert.Contains(t, out, "quoted text")
}

func TestWriteMessageListLocalTimezone(t *testing.T) {
	t.Parallel()
	jst := time.FixedZone("JST", 9*60*60)
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	require.NoError(t, writeMessageList(&buf, messageList{
		label: "#general", limit: 10, order: "desc",
		msgs: []traq.Message{
			{Id: "m1", UserId: "u1", Content: "hi", CreatedAt: base, UpdatedAt: base},
		},
		users:  map[string]traq.User{"u1": {Id: "u1", Name: "alice", DisplayName: "alice"}},
		stamps: map[string]string{}, loc: jst,
	}))
	assert.Contains(t, buf.String(), "2026-10-17T19:00:00+09:00")
}
