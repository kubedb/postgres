package cmds

import (
	"github.com/spf13/cobra"
)

func NewCmdLeaderElection() *cobra.Command {

	cmd := &cobra.Command{
		Use:               "leader_election",
		Short:             "Run leader election for postgres",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {

		},
	}

	return cmd
}
