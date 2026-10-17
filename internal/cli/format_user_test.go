package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/go-traq"
)

func TestWriteMyUserDetail(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	last := base.Add(-time.Hour)
	home := "channel-id"
	me := &traq.MyUserDetail{
		Id:          "user-id",
		Name:        "alice",
		DisplayName: "Alice Smith",
		Bot:         false,
		State:       traq.USERACCOUNTSTATE_active,
		Bio:         "hello",
		TwitterId:   "alice_x",
		Groups:      []string{"g1", "g2"},
		Tags: []traq.UserTag{
			{TagId: "t1", Tag: "tag1"},
			{TagId: "t2", Tag: "tag2", IsLocked: true},
		},
		UpdatedAt:   base,
		LastOnline:  *traq.NewNullableTime(&last),
		IconFileId:  "icon-id",
		Permissions: []traq.UserPermission{"get_webhook"},
		HomeChannel: *traq.NewNullableString(&home),
	}

	var buf bytes.Buffer
	require.NoError(t, writeMyUserDetail(&buf, me, time.UTC))
	out := buf.String()
	assert.Contains(t, out, "@alice (Alice Smith) id:user-id")
	assert.NotContains(t, out, `"id"`)
	assert.Contains(t, out, "state: active")
	assert.Contains(t, out, "bio: hello")
	assert.Contains(t, out, "twitter: alice_x")
	assert.Contains(t, out, "lastOnline: 2026-10-17T09:00:00Z")
	assert.Contains(t, out, "updatedAt: 2026-10-17T10:00:00Z")
	assert.Contains(t, out, "icon: icon-id")
	assert.Contains(t, out, "homeChannel: channel-id")
	assert.Contains(t, out, "groups: g1, g2")
	assert.Contains(t, out, "tags: tag1, tag2 (locked)")
	assert.Contains(t, out, "permissions: get_webhook")
}

func TestWriteMyUserDetailBot(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	me := &traq.MyUserDetail{
		Id:          "bot-id",
		Name:        "BOT_x",
		DisplayName: "BOT_x",
		Bot:         true,
		State:       traq.USERACCOUNTSTATE_active,
		UpdatedAt:   base,
	}

	var buf bytes.Buffer
	require.NoError(t, writeMyUserDetail(&buf, me, time.UTC))
	out := buf.String()
	assert.Contains(t, out, "@BOT_x id:bot-id [bot]")
	assert.NotContains(t, out, "bio:")
}

func TestUserStateLabel(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "active", userStateLabel(traq.USERACCOUNTSTATE_active))
	assert.Equal(t, "deactivated", userStateLabel(traq.USERACCOUNTSTATE_deactivated))
	assert.Equal(t, "suspended", userStateLabel(traq.USERACCOUNTSTATE_suspended))
}
