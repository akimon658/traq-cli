package traq

import (
	"context"
	"fmt"
	"os"
	"strings"
	"uuid"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// ResolveChannelID はチャンネル指定 (UUID または "#general/a/b" 形式のパス) を ID に解決する。
func (c *ClientImpl) ResolveChannelID(ctx context.Context, target string) (string, error) {
	if _, err := uuid.Parse(target); err == nil {
		return target, nil
	}
	path := strings.TrimPrefix(target, "#")
	list, err := c.GetChannels(ctx, true)
	if err != nil {
		return "", err
	}
	channelID, err := FindByPath(list, path)
	if err != nil {
		return "", exit.GeneralError(err)
	}
	if channelID == "" {
		return "", exit.Generalf("channel not found: %s", target)
	}
	return channelID, nil
}

// GetChannelLabel はチャンネルIDから表示用ラベルを返す。
// 公開チャンネルは "#a/b"、DM は "@name"。
// 取得失敗・未解決のときは channelID をそのまま返す (UUID フォールバック)。
func (c *ClientImpl) GetChannelLabel(ctx context.Context, channelID string) string {
	list, err := c.GetChannels(ctx, true)
	if err != nil || list == nil {
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: failed to resolve channel list:", err)
		} else {
			fmt.Fprintln(os.Stderr, "warning: failed to resolve channel list: nil list")
		}
		return channelID
	}
	var userNames map[string]string
	for _, dm := range list.Dm {
		if dm.Id != channelID {
			continue
		}
		names, uerr := c.GetUserNameMap(ctx)
		if uerr != nil {
			fmt.Fprintln(os.Stderr, "warning: failed to resolve user names:", uerr)
			break
		}
		userNames = names
		break
	}
	return FormatChannelLabel(list, userNames, channelID)
}

// GetUserNameMap はユーザーIDから名前へのマップを返す。
func (c *ClientImpl) GetUserNameMap(ctx context.Context) (map[string]string, error) {
	users, err := c.GetUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(users))
	for _, u := range users {
		out[u.Id] = u.Name
	}
	return out, nil
}

// GetStampNameMap はスタンプIDから名前へのマップを返す。
func (c *ClientImpl) GetStampNameMap(ctx context.Context) (map[string]string, error) {
	stamps, err := c.GetStamps(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(stamps))
	for _, s := range stamps {
		out[s.Id] = s.Name
	}
	return out, nil
}

// GetUserByID はユーザーIDからユーザー情報を返す。
// 見つからない場合・取得失敗時はエラーを返し、呼び出し側はIDフォールバック表示に倒す。
func (c *ClientImpl) GetUserByID(ctx context.Context, id string) (traq.User, error) {
	users, err := c.GetUsers(ctx)
	if err != nil {
		return traq.User{}, err
	}
	for _, u := range users {
		if u.Id == id {
			return u, nil
		}
	}
	return traq.User{}, fmt.Errorf("user not found: %s", id)
}

// GetStampName はスタンプIDから名前を返す。
// 見つからない場合・取得失敗時はエラーを返し、呼び出し側はIDフォールバック表示に倒す。
func (c *ClientImpl) GetStampName(ctx context.Context, id string) (string, error) {
	stamps, err := c.GetStamps(ctx)
	if err != nil {
		return "", err
	}
	for _, s := range stamps {
		if s.Id == id {
			return s.Name, nil
		}
	}
	return "", fmt.Errorf("stamp not found: %s", id)
}

// GetSelfIDAndGroups は自分宛メンション判定用の自IDと所属グループを返す。
func (c *ClientImpl) GetSelfIDAndGroups(ctx context.Context) (string, map[string]struct{}, error) {
	me, err := c.GetMe(ctx)
	if err != nil || me == nil {
		return "", map[string]struct{}{}, err
	}
	groups := make(map[string]struct{}, len(me.Groups))
	for _, g := range me.Groups {
		groups[g] = struct{}{}
	}
	return me.Id, groups, nil
}

// ResolveUserIDs は UUID または @name/name を UUID に変換する。
func (c *ClientImpl) ResolveUserIDs(ctx context.Context, inputs []string) ([]string, error) {
	users, err := c.GetUsers(ctx)
	if err != nil {
		return nil, err
	}
	nameToID := make(map[string]string, len(users))
	for _, u := range users {
		nameToID[u.Name] = u.Id
	}
	ids := make([]string, 0, len(inputs))
	for _, in := range inputs {
		if _, err := uuid.Parse(in); err == nil {
			ids = append(ids, in)
			continue
		}
		name := strings.TrimPrefix(in, "@")
		if name == "" {
			return nil, fmt.Errorf("empty user name")
		}
		id, ok := nameToID[name]
		if !ok {
			return nil, fmt.Errorf("user not found: %s", in)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GetDMUserMap は DM チャンネルIDから相手ユーザーIDへのマップを返す。
func (c *ClientImpl) GetDMUserMap(ctx context.Context) (map[string]string, error) {
	list, err := c.GetChannels(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(list.Dm))
	for _, dm := range list.Dm {
		out[dm.Id] = dm.UserId
	}
	return out, nil
}

// GetChannelPathMap は公開チャンネル ID からフルパス ("#a/b" 形式) へのマップを返す。
func (c *ClientImpl) GetChannelPathMap(ctx context.Context) (map[string]string, error) {
	list, err := c.GetChannels(ctx, true)
	if err != nil {
		return nil, err
	}
	return BuildPathMap(list), nil
}

// GetSubscriptionLevels はチャンネルIDから購読レベルへのマップを返す。
func (c *ClientImpl) GetSubscriptionLevels(
	ctx context.Context,
) (map[string]traq.ChannelSubscribeLevel, error) {
	subs, err := c.GetMyChannelSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]traq.ChannelSubscribeLevel, len(subs))
	for _, s := range subs {
		out[s.ChannelId] = s.Level
	}
	return out, nil
}

// GetStarSet はお気に入りチャンネルIDの集合を返す。
func (c *ClientImpl) GetStarSet(ctx context.Context) (map[string]bool, error) {
	ids, err := c.GetMyStars(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}
