package traq

import (
	"context"
	"fmt"
	"strings"
	"uuid"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/foundation/exit"
)

// ResolveChannelID はチャンネル指定 (UUID または "#general/a/b" 形式のパス) を ID に解決する。
func (c *Client) ResolveChannelID(ctx context.Context, target string) (string, error) {
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
func (c *Client) GetChannelLabel(ctx context.Context, channelID string) string {
	list, err := c.GetChannels(ctx, true)
	if err != nil || list == nil {
		return channelID
	}
	var userNames map[string]string
	for _, dm := range list.Dm {
		if dm.Id != channelID {
			continue
		}
		names, uerr := c.GetUserNameMap(ctx)
		if uerr != nil {
			break
		}
		userNames = names
		break
	}
	return FormatChannelLabel(list, userNames, channelID)
}

// GetUserNameMap はユーザーIDから名前へのマップを返す。
func (c *Client) GetUserNameMap(ctx context.Context) (map[string]string, error) {
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
func (c *Client) GetStampNameMap(ctx context.Context) (map[string]string, error) {
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

// GetSelfIDAndGroups は自分宛メンション判定用の自IDと所属グループを返す。
func (c *Client) GetSelfIDAndGroups(ctx context.Context) (string, map[string]struct{}, error) {
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
func (c *Client) ResolveUserIDs(ctx context.Context, inputs []string) ([]string, error) {
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
func (c *Client) GetDMUserMap(ctx context.Context) (map[string]string, error) {
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
func (c *Client) GetChannelPathMap(ctx context.Context) (map[string]string, error) {
	list, err := c.GetChannels(ctx, true)
	if err != nil {
		return nil, err
	}
	return BuildPathMap(list), nil
}

// GetSubscriptionLevels はチャンネルIDから購読レベルへのマップを返す。
func (c *Client) GetSubscriptionLevels(
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
func (c *Client) GetStarSet(ctx context.Context) (map[string]bool, error) {
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
