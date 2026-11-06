package traq

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akimon658/traq-cli/internal/domain/auth"
	dtraq "github.com/akimon658/traq-cli/internal/domain/traq"
)

func TestClient_GetMe(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		me, err := testBotClient(t).GetMe(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "BOT_"+testBotName, me.Name)
		assert.True(t, me.Bot)
	})

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		c := NewService(testTraqOrigin, "test", nil, nil).
			WithToken(&ResolvedToken{Token: &auth.Token{AccessToken: "invalid"}})
		_, err := c.GetMe(t.Context())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the token was rejected")
	})
}

func TestClient_PostMessage(t *testing.T) {
	t.Run("by path and by uuid", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		c := testBotClient(t)
		ctx := t.Context()

		channelID, err := c.ResolveChannelID(ctx, "#general")
		require.NoError(t, err)
		require.NotEmpty(t, channelID)

		for _, tc := range []struct {
			name   string
			target string
		}{
			{"path", "#general"},
			{"uuid", channelID},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				content := "hello traq-cli " + uuid.New().String()
				id, err := c.ResolveChannelID(ctx, tc.target)
				require.NoError(t, err)
				msg, err := c.PostMessage(ctx, id, content)
				require.NoError(t, err)
				assert.Equal(t, content, msg.Content)

				got, err := c.GetMessage(ctx, msg.Id)
				require.NoError(t, err)
				assert.Equal(t, content, got.Content)
			})
		}
	})

	t.Run("channel not found", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		_, err := testBotClient(t).ResolveChannelID(t.Context(), "#does-not-exist")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel not found")
	})
}

func TestClient_GetMessage(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		c := testBotClient(t)
		ctx := t.Context()
		channelID, err := c.ResolveChannelID(ctx, "#general")
		require.NoError(t, err)
		content := "get-" + strings.ReplaceAll(uuid.New().String(), "-", "")
		posted, err := c.PostMessage(ctx, channelID, content)
		require.NoError(t, err)

		got, err := c.GetMessage(ctx, posted.Id)
		require.NoError(t, err)
		assert.Equal(t, content, got.Content)
		assert.Equal(t, posted.Id, got.Id)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		_, err := testBotClient(t).GetMessage(t.Context(), uuid.New().String())
		require.Error(t, err)
	})
}

func TestClient_GetChannelMessages(t *testing.T) {
	t.Run("ok desc", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		c := testBotClient(t)
		ctx := t.Context()
		channelID, err := c.ResolveChannelID(ctx, "#general")
		require.NoError(t, err)

		tag := strings.ReplaceAll(uuid.New().String(), "-", "")
		first := "list-" + tag + "-first"
		second := "list-" + tag + "-second"
		for _, content := range []string{first, second} {
			_, err := c.PostMessage(ctx, channelID, content)
			require.NoError(t, err)
		}

		msgs, err := c.GetChannelMessages(ctx, channelID, dtraq.MessageQuery{
			Limit: 100, Order: "desc",
		})
		require.NoError(t, err)
		var got []string
		for _, m := range msgs {
			got = append(got, m.Content)
		}
		assert.Contains(t, got, first)
		assert.Contains(t, got, second)
	})
}

func TestClient_SearchMessages(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		c := testBotClient(t)
		ctx := t.Context()
		channelID, err := c.ResolveChannelID(ctx, "#general")
		require.NoError(t, err)

		tag := strings.ReplaceAll(uuid.New().String(), "-", "")
		content := "search-" + tag
		_, err = c.PostMessage(ctx, channelID, content)
		require.NoError(t, err)

		var hits []string
		require.Eventually(t, func() bool {
			res, err := c.SearchMessages(ctx, dtraq.SearchQuery{Limit: 10, Sort: "-createdAt"})
			if err != nil {
				t.Logf("search retry: %v", err)
				return false
			}
			hits = hits[:0]
			for _, m := range res.Hits {
				hits = append(hits, m.Content)
			}
			for _, h := range hits {
				if strings.Contains(h, content) {
					return true
				}
			}
			return false
		}, 90*time.Second, 2*time.Second, "search hit not found for %q", content)
	})

	t.Run("in and from", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		c := testBotClient(t)
		ctx := t.Context()
		channelID, err := c.ResolveChannelID(ctx, "#general")
		require.NoError(t, err)

		tag := strings.ReplaceAll(uuid.New().String(), "-", "")
		content := "searchfilter-" + tag
		_, err = c.PostMessage(ctx, channelID, content)
		require.NoError(t, err)

		ids, err := c.ResolveUserIDs(ctx, []string{"@BOT_traq-cli"})
		require.NoError(t, err)
		require.NotEmpty(t, ids)
	})
}

func TestClient_ResolveChannelID(t *testing.T) {
	t.Run("uuid passthrough", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		id := uuid.New().String()
		got, err := testBotClient(t).ResolveChannelID(t.Context(), id)
		require.NoError(t, err)
		assert.Equal(t, id, got)
	})

	t.Run("path", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		got, err := testBotClient(t).ResolveChannelID(t.Context(), "#general")
		require.NoError(t, err)
		assert.Equal(t, testGeneralChannelID(t), got)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		requireTraqStack(t)

		_, err := testBotClient(t).ResolveChannelID(t.Context(), "#does-not-exist")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel not found")
	})
}
