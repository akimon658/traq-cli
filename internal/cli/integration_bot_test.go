package cli

import (
	"regexp"
	"strings"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/auth"
)

var messageIDPattern = regexp.MustCompile(`id:([0-9a-fA-F-]{36})`)

func TestUsersMe(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	out, err := runCLI(t, botEnv(t, testTraqToken), "user", "me")
	require.NoError(t, err, "users me failed")
	assert.NotContains(t, out, `"id"`, "output should be human-readable text, not JSON")
	apiMe := apiGetJSON(t, "/users/me")
	assert.Contains(t, out, "id:"+asString(apiMe["id"]))
	assert.Contains(t, out, "@"+asString(apiMe["name"]))
	assert.Contains(t, strings.ToLower(out), testBotName)
}

func TestUsersMeInvalidToken(t *testing.T) {
	t.Parallel()
	requireTraqStack(t)

	_, err := runCLI(t, botEnv(t, "invalid"), "user", "me")
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
	tag := strings.ReplaceAll(uuid.New().String(), "-", "")
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
	_, err := runCLI(t, e, "message", "send", "#general", "list-limit-"+uuid.New().String())
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
	assert.NotContains(t, out, `"id"`, "output should be human-readable text, not JSON")
	assert.Contains(t, out, wantContent)
	m := messageIDPattern.FindStringSubmatch(out)
	require.Len(t, m, 2, "no message id in the output: %s", out)
	got := apiGetJSON(t, "/messages/"+m[1])
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
	assert.NotContains(t, out, `"origin"`, "output should be human-readable text, not JSON")
	me := apiGetJSON(t, "/users/me")
	for _, want := range []string{
		"origin: " + testTraqOrigin,
		"source: environment",
		"method: bot",
		"userId: " + asString(me["id"]),
		"user: @" + asString(me["name"]),
	} {
		assert.Contains(t, out, want, "output:\n%s", out)
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
