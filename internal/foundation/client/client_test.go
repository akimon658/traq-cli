package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/go-traq"
)

func TestGetUsersIncludesSuspended(t *testing.T) {
	t.Parallel()
	const body = `[{
		"id": "u1",
		"name": "alice",
		"displayName": "Alice",
		"iconFileId": "",
		"bot": false,
		"state": 2,
		"updatedAt": "2026-10-17T10:00:00Z"
	}]`

	queries := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/users" {
			http.NotFound(w, r)
			return
		}
		queries <- r.URL.Query().Get("include-suspended")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := New(srv.URL)
	users, _, err := c.GetUsers(TokenContext(t.Context(), "token"))
	require.NoError(t, err)
	assert.Equal(t, "true", <-queries,
		"include-suspended=true must be sent to resolve suspended users")
	require.Len(t, users, 1)
	assert.Equal(t, "u1", users[0].Id)
	assert.Equal(t, traq.USERACCOUNTSTATE_suspended, users[0].State)
	assert.Equal(t, time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC), users[0].UpdatedAt)
}
