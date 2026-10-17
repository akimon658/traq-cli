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
		"  :ok: ×1",
		"  :thank: ×3",
		"",
	}, "\n")
	assert.Equal(t, want, buf.String())
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

func TestWriteMessageListUserNotFound(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	err := writeMessageList(&buf, messageList{
		label: "#general", limit: 10, order: "desc",
		msgs:   []traq.Message{{Id: "m1", UserId: "missing", CreatedAt: base, UpdatedAt: base}},
		users:  map[string]traq.User{},
		stamps: map[string]string{}, loc: time.UTC,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
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
