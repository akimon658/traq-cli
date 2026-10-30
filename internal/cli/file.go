package cli

import (
	"github.com/spf13/cobra"

	"github.com/akimon658/traq-cli/internal/client"
	"github.com/akimon658/traq-cli/internal/config"
	"github.com/akimon658/traq-cli/internal/exit"
	"github.com/akimon658/traq-cli/internal/file"
	"github.com/akimon658/traq-cli/internal/session"
	"github.com/akimon658/traq-cli/internal/view"
)

func newFileCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
	fileCmd := &cobra.Command{
		Use:     "file",
		Aliases: []string{"files"},
		Short:   "ファイル操作サブコマンド",
	}
	fileCmd.AddCommand(newFileDownloadCmd(newEnv, flags))
	return fileCmd
}

func newFileDownloadCmd(newEnv func() *config.Env, flags *globalFlags) *cobra.Command {
	var (
		out   string
		force bool
	)
	downloadCmd := &cobra.Command{
		Use:   "download <file-id>",
		Short: "ファイルをダウンロードする",
		Long: `<file-id> はファイル UUID です。
--out を省略するとカレントディレクトリに元のファイル名で保存します。
--out に既存のディレクトリを指定すると、その中に元のファイル名で保存します。
保存先の親ディレクトリは自動作成しません。
既存ファイルは --force を指定しない限り上書きしません。`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return exit.UsageError(err)
			}
			if err := file.ValidateFileID(args[0]); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			e := newEnv()
			flags.apply(cmd, e)
			rt, err := session.ResolveToken(cmd.Context(), e)
			if err != nil {
				return err
			}
			c := client.New(e.Origin)
			res, err := file.Download(cmd.Context(), e, &rt, c, args[0], out, force)
			if err != nil {
				return err
			}
			return view.WriteFileSaved(cmd.OutOrStdout(), res)
		},
	}
	downloadCmd.Flags().
		StringVar(&out, "out", "", "保存先のファイルまたはディレクトリ (省略時はカレントディレクトリ)")
	downloadCmd.Flags().BoolVar(&force, "force", false, "既存ファイルを上書きする")
	return downloadCmd
}
