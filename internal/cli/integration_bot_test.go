package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/akimon658/traq-cli/internal/auth"
)

func TestUsersMe(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", testTraqToken)

	out, err := runCLI(t, "users", "me")
	require.NoError(t, err, "users me failed")
	var me map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &me), "output is not JSON: %s", out)
	apiMe := apiGetJSON(t, "/users/me")
	assert.Equal(t, apiMe["id"], me["id"])
	assert.Contains(t, strings.ToLower(asString(me["name"])), testBotName)
}

func TestUsersMeInvalidToken(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", "invalid")

	_, err := runCLI(t, "users", "me")
	require.Error(t, err, "expected an error for the invalid token")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
	assert.Contains(t, err.Error(), "the token was rejected")
}

func TestMessageSend(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", testTraqToken)

	const (
		contentByPath = "hello traq-cli (by #general path)"
		contentByUUID = "hello traq-cli (by channel UUID)"
	)

	out, err := runCLI(t, "message", "send", "#general", contentByPath)
	require.NoError(t, err, "message send failed")
	assertPosted(t, out, contentByPath)

	channelID := testGeneralChannelID(t)
	out, err = runCLI(t, "message", "send", channelID, contentByUUID)
	require.NoError(t, err, "message send failed")
	assertPosted(t, out, contentByUUID)
}

func TestMessageSendChannelNotFound(t *testing.T) {
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", testTraqToken)

	out, err := runCLI(t, "message", "send", "#does-not-exist", "hello")
	require.Error(t, err, "expected an error for the nonexistent channel, output: %s", out)
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)
	assert.Contains(t, err.Error(), "channel not found")
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
	requireTraqStack(t)
	t.Setenv("TRAQ_ORIGIN", testTraqOrigin)
	t.Setenv("TRAQ_ACCESS_TOKEN", testTraqToken)

	out, err := runCLI(t, "auth", "status")
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
	_, err := runCLI(t, "auth", "login")
	require.Error(t, err, "auth login without --browser or --token should fail")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitUsage, ec.Code)
	assert.Contains(t, err.Error(), "specify either --browser or --token")
}

func TestAuthLoginTokenAndLogout(t *testing.T) {
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
	overrideEnv(t, ms)

	out, err := runCLI(t, "auth", "login", "--token", testTraqToken)
	require.NoError(t, err, "auth login failed")
	assert.Contains(t, out, "signed in: @")
	tok := saved
	assert.Equal(t, testTraqToken, tok.AccessToken)
	assert.Equal(t, auth.MethodBot, tok.Method)
	assert.NotEmpty(t, tok.UserName, "userName is empty")
	assert.Equal(t, testTraqOrigin, tok.Origin)

	out, err = runCLI(t, "auth", "logout")
	require.NoError(t, err, "auth logout failed")
	assert.Contains(t, out, "logged out")

	_, err = runCLI(t, "auth", "logout")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitAuthRequired, ec.Code)
}

func TestAuthLoginInvalidToken(t *testing.T) {
	requireTraqStack(t)
	ms := auth.NewMockStore(gomock.NewController(t))
	overrideEnv(t, ms)

	_, err := runCLI(t, "auth", "login", "--token", "invalid")
	require.Error(t, err, "expected an error for the invalid token")
	var ec *ExitCodeError
	require.ErrorAs(t, err, &ec)
	assert.Equal(t, exitGeneral, ec.Code)
	assert.Contains(t, err.Error(), "the token was rejected")
}
