package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/traPtitech/go-traq"
)

// writeMyUserDetail は自分のユーザー詳細を可読性の高いテキストで出力する。
func writeMyUserDetail(w io.Writer, me *traq.MyUserDetail, loc *time.Location) error {
	if loc == nil {
		loc = time.Local
	}

	header := fmt.Sprintf("%s id:%s", myUserLabel(me), me.Id)
	if me.Bot {
		header += " [bot]"
	}
	lines := []string{header}
	lines = append(lines, fmt.Sprintf("state: %s", userStateLabel(me.State)))
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

// myUserLabel は MyUserDetail 用の "@name (DisplayName)" ラベルを組み立てる。
func myUserLabel(me *traq.MyUserDetail) string {
	label := "@" + me.Name
	if me.DisplayName != "" && me.DisplayName != me.Name {
		label += " (" + me.DisplayName + ")"
	}
	return label
}

// userStateLabel はアカウント状態を可読名で返す。
func userStateLabel(state traq.UserAccountState) string {
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
