package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/auth"
)

func TestUsersMe(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	out, err := runCLI(t, botEnv(t, testTraqToken), "users", "me")
	require.NoError(t, err, "users me failed")
	var me map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &me), "output is not JSON: %s", out)
	apiMe := apiGetJSON(t, "/users/me")
	assert.Equal(t, apiMe["id"], me["id"])
	assert.Contains(t, strings.ToLower(asString(me["name"])), testBotName)
}

func TestUsersMeInvalidToken(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	_, err := runCLI(t, botEnv(t, "invalid"), "users", "me")
	require.Error(t, err, "expected an error for the invalid token")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
	assert.Contains(t, err.Error(), "the token was rejected")
}

func TestMessageSend(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	const (
		contentByPath = "hello traq-cli (by #general path)"
		contentByUUID = "hello traq-cli (by channel UUID)"
	)

	e := botEnv(t, testTraqToken)
	out, err := runCLI(t, e, "message", "send", "#general", contentByPath)
	require.NoError(t, err, "message send failed")
	assertPosted(t, out, contentByPath)

	channelID := testGeneralChannelID(t)
	out, err = runCLI(t, e, "message", "send", channelID, contentByUUID)
	require.NoError(t, err, "message send failed")
	assertPosted(t, out, contentByUUID)
}

func TestMessageSendChannelNotFound(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	out, err := runCLI(t, botEnv(t, testTraqToken), "message", "send", "#does-not-exist", "hello")
	require.Error(t, err, "expected an error for the nonexistent channel, output: %s", out)
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)
	assert.Contains(t, err.Error(), "channel not found")
}

func TestMessageList(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	e := botEnv(t, testTraqToken)
	tag := strings.ReplaceAll(uuid.NewString(), "-", "")
	first := "list-" + tag + "-first"
	second := "list-" + tag + "-second"
	for _, content := range []string{first, second} {
		_, err := runCLI(t, e, "message", "send", "#general", content)
		require.NoError(t, err, "message send failed")
	}

	out, err := runCLI(
		t, e, "message", "list", "#general", "--limit", "100", "--show-unread=false",
	)
	require.NoError(t, err, "message list failed")
	assert.Contains(t, out, "#general (")
	assert.Contains(t, out, "order=desc")
	assert.Contains(t, out, "@BOT_traq-cli", "the author name should be resolved")
	assert.NotContains(t, out, `"id"`, "output should be human-readable text, not JSON")
	i1 := strings.Index(out, first)
	i2 := strings.Index(out, second)
	require.NotEqual(t, -1, i1, "first message not found in:\n%s", out)
	require.NotEqual(t, -1, i2, "second message not found in:\n%s", out)
	assert.Less(t, i1, i2, "messages should be displayed oldest first")
}

func TestMessageListLimitSummary(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	e := botEnv(t, testTraqToken)
	_, err := runCLI(t, e, "message", "send", "#general", "list-limit-"+uuid.NewString())
	require.NoError(t, err, "message send failed")

	out, err := runCLI(
		t, e, "message", "list", "#general", "--limit", "1", "--show-unread=false",
	)
	require.NoError(t, err, "message list failed")
	assert.Contains(t, out, "(1件 limit=1 offset=0 order=desc)")
}

func TestMessageListInvalidFlags(t *testing.T) {
	t.Parallel()

	e := botEnv(t, "")
	cases := map[string][]string{
		"limit":  {"message", "list", "#general", "--limit", "0"},
		"offset": {"message", "list", "#general", "--offset=-1"},
		"order":  {"message", "list", "#general", "--order", "up"},
		"since":  {"message", "list", "#general", "--since", "not-a-time"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runCLI(t, e, args...)
			var ec *ExitCodeError
			require.ErrorAs(t, err, &ec)
			assert.Equal(t, exitUsage, ec.Code)
		})
	}
}

func assertPosted(t *testing.T, out, wantContent string) {
	t.Helper()
	var msg map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &msg), "output is not JSON: %s", out)
	id, ok := msg["id"].(string)
	require.True(t, ok && id != "", "no message id in the output: %s", out)
	got := apiGetJSON(t, "/messages/"+id)
	assert.Equal(t, wantContent, asString(got["content"]))
}

func testGeneralChannelID(t *testing.T) string {
	t.Helper()
	list := apiGetJSON(t, "/channels")
	pub, ok := list["public"].([]any)
	require.True(t, ok, "unexpected channels response: %v", list)
	for _, ch := range pub {
		m, ok := ch.(map[string]any)
		if !ok {
			continue
		}
		name, id := asString(m["name"]), asString(m["id"])
		if name == "general" && id != "" {
			return id
		}
	}
	require.Fail(t, "channel general not found in the response", "%v", list)
	return ""
}

func TestAuthStatus(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	out, err := runCLI(t, botEnv(t, testTraqToken), "auth", "status")
	require.NoError(t, err, "auth status failed")
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &status), "output is not JSON: %s", out)
	me := apiGetJSON(t, "/users/me")
	for key, want := range map[string]any{
		"origin": testTraqOrigin,
		"source": "environment",
		"method": "bot",
		"userId": asString(me["id"]),
		"user":   "@" + asString(me["name"]),
	} {
		assert.Equal(t, want, status[key], "status[%q]", key)
	}
}

func TestAuthLoginUsage(t *testing.T) {
	t.Parallel()

	_, err := runCLI(t, botEnv(t, ""), "auth", "login")
	require.Error(t, err, "auth login without --browser or --token should fail")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitUsage, ec.Code)
	assert.Contains(t, err.Error(), "specify either --browser, --manual or --token")
}

func TestAuthLoginTokenAndLogout(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	ms := auth.NewMockStore(gomock.NewController(t))
	var saved auth.Token
	gomock.InOrder(
		ms.EXPECT().Save(gomock.Any()).DoAndReturn(func(tok auth.Token) (string, error) {
			saved = tok
			return "keychain", nil
		}).Times(1),
		ms.EXPECT().Load().DoAndReturn(func() (auth.Token, string, bool, error) {
			return saved, "keychain", true, nil
		}).Times(1),
		ms.EXPECT().Delete().Return(true, nil).Times(1),
		ms.EXPECT().Load().Return(auth.Token{}, "", false, nil).Times(1),
	)
	e := overrideEnv(t, ms)

	out, err := runCLI(t, e, "auth", "login", "--token", testTraqToken)
	require.NoError(t, err, "auth login failed")
	assert.Contains(t, out, "signed in: @")
	tok := saved
	assert.Equal(t, testTraqToken, tok.AccessToken)
	assert.Equal(t, auth.MethodBot, tok.Method)
	assert.NotEmpty(t, tok.UserName, "userName is empty")
	assert.Equal(t, testTraqOrigin, tok.Origin)

	out, err = runCLI(t, e, "auth", "logout")
	require.NoError(t, err, "auth logout failed")
	assert.Contains(t, out, "logged out")

	_, err = runCLI(t, e, "auth", "logout")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
}

func TestAuthLoginInvalidToken(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	ms := auth.NewMockStore(gomock.NewController(t))
	e := overrideEnv(t, ms)

	_, err := runCLI(t, e, "auth", "login", "--token", "invalid")
	require.Error(t, err, "expected an error for the invalid token")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)
	assert.Contains(t, err.Error(), "the token was rejected")
}
