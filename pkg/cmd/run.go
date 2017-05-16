package cmd

import (
	"fmt"
	"time"

	"github.com/k8sdb/postgres/pkg/controller"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/util/runtime"
)

const (
	// Default tag
	canary = "canary-util"
)

func NewCmdRun() *cobra.Command {
	var (
		masterURL       string
		kubeconfigPath  string
		postgresUtilTag string
		governingService string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run Postgres in Kubernetes",
		Run: func(cmd *cobra.Command, args []string) {
			config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				fmt.Printf("Could not get kubernetes config: %s", err)
				time.Sleep(30 * time.Minute)
				panic(err)
			}
			defer runtime.HandleCrash()

			w := controller.New(config, postgresUtilTag, governingService)
			fmt.Println("Starting operator...")
			w.RunAndHold()
		},
	}
	cmd.Flags().StringVar(&masterURL, "master", "", "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	cmd.Flags().StringVar(&postgresUtilTag, "postgres-util", canary, "Tag of postgres util")
	cmd.Flags().StringVar(&governingService, "governing-service", "k8sdb", "Governing service for database statefulset")

	return cmd
}
