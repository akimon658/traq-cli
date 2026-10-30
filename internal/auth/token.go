package auth

import (
	"time"

	"github.com/traPtitech/go-traq"
)

type Method string

const (
	MethodBot   Method = "bot"
	MethodOAuth Method = "oauth"
)

const (
	ScopeRead  = "read"
	ScopeWrite = "write"
)

type Token struct {
	Method       Method    `json:"method"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	Scope        []string  `json:"scope,omitempty"`
	Origin       string    `json:"origin,omitempty"`
	UserID       string    `json:"userId,omitempty"`
	UserName     string    `json:"userName,omitempty"`
	UserBot      bool      `json:"userBot,omitempty"`
	IssuedAt     time.Time `json:"issuedAt,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}

func (t *Token) LacksScope(scope string) bool {
	if t.Method != MethodOAuth || len(t.Scope) == 0 {
		return false
	}
	return !ContainsScope(t.Scope, scope)
}

func (t *Token) Expired() bool {
	return !t.ExpiresAt.IsZero() && t.ExpiresAt.Before(time.Now())
}

func ContainsScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// buildTokenFromMe は /users/me の応答から保存用 Token を組み立てる。
// bot と OAuth の種別判別を一箇所に集約するためのヘルパー。
func buildTokenFromMe(origin, accessToken string, me *traq.MyUserDetail) Token {
	t := Token{
		AccessToken: accessToken,
		Origin:      origin,
		UserID:      me.Id,
		UserName:    me.Name,
		UserBot:     me.Bot,
		IssuedAt:    time.Now(),
	}
	if me.Bot {
		t.Method = MethodBot
	} else {
		t.Method = MethodOAuth
	}
	return t
}
