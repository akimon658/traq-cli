package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/traPtitech/go-traq"

	"github.com/akimon658/traq-cli/internal/auth"
	"github.com/akimon658/traq-cli/internal/client"
)

var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "メッセージ操作サブコマンド",
}

const minSendArgs = 2

var sendMessageCmd = &cobra.Command{
	Use:   "send <チャンネル> <本文>...",
	Short: "チャンネルにメッセージを投稿する",
	Long: `<チャンネル> はチャンネル UUID、または "#general/a/b" 形式のチャンネルパスで指定します。
"#" は省略できます。`,
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.MinimumNArgs(minSendArgs)(cmd, args); err != nil {
			return &ExitCodeError{exitUsage, err}
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		e := newEnv()
		rt, err := e.resolveToken()
		if err != nil {
			return err
		}
		if err := e.requireScope(&rt, auth.ScopeWrite); err != nil {
			return err
		}
		c := client.New(e.Origin)
		ctx := context.Background()
		channelID, err := resolveChannel(e, &rt, c, ctx, args[0])
		if err != nil {
			return err
		}
		content := strings.Join(args[1:], " ")
		var msg *traq.Message
		err = e.apiExec(ctx, &rt, func(ctx context.Context) (*http.Response, error) {
			var resp *http.Response
			msg, resp, err = c.API.MessageAPI.
				PostMessage(ctx, channelID).
				PostMessageRequest(traq.PostMessageRequest{Content: content}).
				Execute()
			return resp, err
		})
		if err != nil {
			return err
		}
		return printJSON(msg)
	},
}

func init() {
	messageCmd.AddCommand(sendMessageCmd)
}

func resolveChannel(
	e *Env,
	rt *ResolvedToken,
	c *client.Client,
	ctx context.Context,
	target string,
) (string, error) {
	if _, err := uuid.Parse(target); err == nil {
		return target, nil
	}
	if rt.Token.LacksScope(auth.ScopeRead) {
		return "", e.requireScope(rt, auth.ScopeRead)
	}
	path := strings.TrimPrefix(target, "#")
	var channelID string
	err := e.apiExec(ctx, rt, func(ctx context.Context) (*http.Response, error) {
		list, resp, apiErr := c.API.ChannelAPI.GetChannels(ctx).Execute()
		if apiErr != nil {
			return resp, apiErr
		}
		channelID, apiErr = findChannelByPath(list, path)
		return resp, apiErr
	})
	if err != nil {
		return "", err
	}
	if channelID == "" {
		return "", &ExitCodeError{exitGeneral, fmt.Errorf("channel not found: %s", target)}
	}
	return channelID, nil
}

func findChannelByPath(list *traq.ChannelList, path string) (string, error) {
	byID := make(map[string]traq.Channel, len(list.Public))
	parent := make(map[string]string, len(list.Public))
	for _, ch := range list.Public {
		byID[ch.Id] = ch
		parent[ch.Id] = ""
		if p := ch.ParentId.Get(); p != nil {
			parent[ch.Id] = *p
		}
	}
	full := make(map[string]string, len(list.Public))
	for id := range byID {
		var segs []string
		cur := id
		for i := 0; i <= len(byID); i++ {
			ch, ok := byID[cur]
			if !ok {
				segs = nil
				break
			}
			segs = append([]string{ch.Name}, segs...)
			cur = parent[cur]
			if cur == "" || cur == id {
				break
			}
		}
		if segs == nil {
			continue
		}
		full[strings.Join(segs, "/")] = id
	}
	if id, ok := full[path]; ok {
		return id, nil
	}
	return "", fmt.Errorf("channel not found: %s", path)
}
