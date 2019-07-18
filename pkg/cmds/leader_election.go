package cmds

import (
	"github.com/spf13/cobra"
	le "kubedb.dev/postgres/pkg/leader_election"
)

func NewCmdLeaderElection() *cobra.Command {

	cmd := &cobra.Command{
		Use:               "leader_election",
		Short:             "Run leader election for postgres",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			le.RunLeaderElection()
		},
	}

	return cmd
}
