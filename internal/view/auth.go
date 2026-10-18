package view

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/akimon658/traq-cli/internal/auth"
)

// WriteAuthStatus は現在の認証状態を可読性の高いテキストで出力する。
func WriteAuthStatus(w io.Writer, origin, source string, t auth.Token) error {
	lines := []string{
		fmt.Sprintf("origin: %s", origin),
		fmt.Sprintf("source: %s", source),
	}
	if method := AuthStatusMethod(&t); method != "" {
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

// AuthStatusMethod は表示用の認証種別を返す。不明な場合は空文字。
func AuthStatusMethod(t *auth.Token) string {
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

// MethodLabel は bot フラグと method から表示用ラベルを返す。
func MethodLabel(m auth.Method, bot bool) string {
	if bot {
		return "bot"
	}
	if m == auth.MethodOAuth {
		return "user"
	}
	return "unknown"
}

// WriteLoginSuccess はトークン登録完了の1行を出力する。
func WriteLoginSuccess(w io.Writer, t auth.Token, venue string) error {
	_, err := fmt.Fprintf(
		w,
		"signed in: @%s (%s)\nstorage: %s\n",
		t.UserName,
		MethodLabel(t.Method, t.UserBot),
		venue,
	)
	return err
}

// WriteLoginSuccessWithScopes は OAuth ログイン完了の1行 (スコープ付き) を出力する。
func WriteLoginSuccessWithScopes(w io.Writer, t auth.Token, venue string) error {
	_, err := fmt.Fprintf(
		w,
		"signed in: @%s (%s) scopes: %s\nstorage: %s\n",
		t.UserName,
		MethodLabel(t.Method, t.UserBot),
		strings.Join(t.Scope, ","),
		venue,
	)
	return err
}

// WriteLogout はログアウト完了を出力する。
func WriteLogout(w io.Writer) error {
	_, err := fmt.Fprintln(w, "logged out")
	return err
}

// WriteManualInstructions は --manual の認可 URL と完了手順を出力する。
func WriteManualInstructions(w io.Writer, authorizeURL string, sessionTTL time.Duration) error {
	if _, err := fmt.Fprintln(w, "Open this URL in a browser to authorize traq-cli:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, authorizeURL); err != nil {
		return err
	}
	_, err := fmt.Fprintf(
		w,
		"The page after authorization will fail to load; this is expected.\n"+
			"Copy the URL from the browser's address bar and run:\n"+
			"  traq auth complete \"<URL>\"\n"+
			"(the one-time code is valid for about 5 minutes; the pending login expires in %s)\n",
		sessionTTL,
	)
	return err
}

// WriteBrowserInstructions は --browser の認可 URL を出力する。
func WriteBrowserInstructions(w io.Writer, authorizeURL string) error {
	if _, err := fmt.Fprintln(w, "Open this URL to authorize traq-cli:"); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, authorizeURL)
	return err
}
