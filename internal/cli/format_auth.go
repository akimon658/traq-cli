package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/akimon658/traq-cli/internal/auth"
)

// writeAuthStatus は現在の認証状態を可読性の高いテキストで出力する。
func writeAuthStatus(w io.Writer, origin, source string, t auth.Token) error {
	lines := []string{
		fmt.Sprintf("origin: %s", origin),
		fmt.Sprintf("source: %s", source),
	}
	if method := authStatusMethod(&t); method != "" {
		lines = append(lines, fmt.Sprintf("method: %s", method))
	}
	if t.UserName != "" {
		lines = append(lines, fmt.Sprintf("user: @%s", t.UserName))
	}
	if t.UserID != "" {
		lines = append(lines, fmt.Sprintf("userId: %s", t.UserID))
	}
	if t.Method == auth.MethodOAuth && len(t.Scope) > 0 {
		lines = append(lines, fmt.Sprintf("scopes: %s", strings.Join(t.Scope, ",")))
	}
	if !t.ExpiresAt.IsZero() {
		lines = append(lines, fmt.Sprintf("expiresAt: %s", t.ExpiresAt.Format(time.RFC3339)))
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// authStatusMethod は表示用の認証種別を返す。不明な場合は空文字。
func authStatusMethod(t *auth.Token) string {
	if t.UserBot {
		return string(auth.MethodBot)
	}
	switch t.Method {
	case auth.MethodBot, auth.MethodOAuth:
		return string(t.Method)
	default:
		return ""
	}
}
