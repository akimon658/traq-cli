package view

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/domain/user"
)

// WriteMyUserDetail は自分のユーザー詳細を可読性の高いテキストで出力する。
func WriteMyUserDetail(w io.Writer, me *traq.MyUserDetail, loc *time.Location) error {
	if loc == nil {
		loc = time.Local
	}

	header := fmt.Sprintf("%s id:%s", MyUserLabel(me), me.Id)
	if me.Bot {
		header += " [bot]"
	}
	lines := []string{header}
	lines = append(lines, fmt.Sprintf("state: %s", UserStateLabel(me.State)))
	if me.Bio != "" {
		lines = append(lines, fmt.Sprintf("bio: %s", me.Bio))
	}
	if me.TwitterId != "" {
		lines = append(lines, fmt.Sprintf("twitter: %s", me.TwitterId))
	}
	if t := me.LastOnline.Get(); t != nil {
		lines = append(lines, fmt.Sprintf("lastOnline: %s", t.In(loc).Format(time.RFC3339)))
	}
	lines = append(lines, fmt.Sprintf("updatedAt: %s", me.UpdatedAt.In(loc).Format(time.RFC3339)))
	if me.IconFileId != "" {
		lines = append(lines, fmt.Sprintf("icon: %s", me.IconFileId))
	}
	if ch := me.HomeChannel.Get(); ch != nil && *ch != "" {
		lines = append(lines, fmt.Sprintf("homeChannel: %s", *ch))
	}
	if len(me.Groups) > 0 {
		lines = append(lines, fmt.Sprintf("groups: %s", strings.Join(me.Groups, ", ")))
	}
	if len(me.Tags) > 0 {
		tags := make([]string, 0, len(me.Tags))
		for _, tag := range me.Tags {
			if tag.IsLocked {
				tags = append(tags, tag.Tag+" (locked)")
			} else {
				tags = append(tags, tag.Tag)
			}
		}
		lines = append(lines, fmt.Sprintf("tags: %s", strings.Join(tags, ", ")))
	}
	if len(me.Permissions) > 0 {
		perms := make([]string, 0, len(me.Permissions))
		for _, p := range me.Permissions {
			perms = append(perms, string(p))
		}
		lines = append(lines, fmt.Sprintf("permissions: %s", strings.Join(perms, ", ")))
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

// MyUserLabel は MyUserDetail 用の "@name (DisplayName)" ラベルを組み立てる。
func MyUserLabel(me *traq.MyUserDetail) string {
	label := "@" + me.Name
	if me.DisplayName != "" && me.DisplayName != me.Name {
		label += " (" + me.DisplayName + ")"
	}
	return label
}

// UserLabel は User 用の "@name (DisplayName)" ラベルを組み立てる。
func UserLabel(u traq.User) string {
	label := "@" + u.Name
	if u.DisplayName != "" && u.DisplayName != u.Name {
		label += " (" + u.DisplayName + ")"
	}
	return label
}

// UserDisplay はユーザーIDから表示用ラベルを返す。
// ユーザーが解決できない場合 (停止済みユーザーが一覧に無い等) は
// 実在しうる "@name" 形式と衝突しない取得失敗表示にフォールバックする。
func UserDisplay(id string, users map[string]traq.User) string {
	if u, ok := users[id]; ok {
		return UserLabel(u)
	}
	return UserFallbackLabel(id)
}

// UserFallbackLabel はユーザーIDが解決できなかったときの表示を返す。
// 形式の正規の定義は domain/user.FallbackLabel。
func UserFallbackLabel(id string) string {
	return user.FallbackLabel(id)
}

// UserStateLabel はアカウント状態を可読名で返す。
func UserStateLabel(state traq.UserAccountState) string {
	switch state {
	case traq.USERACCOUNTSTATE_active:
		return "active"
	case traq.USERACCOUNTSTATE_deactivated:
		return "deactivated"
	case traq.USERACCOUNTSTATE_suspended:
		return "suspended"
	default:
		return fmt.Sprintf("%d", int32(state))
	}
}
