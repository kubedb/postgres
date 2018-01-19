package cmds

import (
	"flag"
	"log"
	"strings"

	"github.com/appscode/go/log/golog"
	v "github.com/appscode/go/version"
	"github.com/jpillora/go-ogle-analytics"
	"github.com/kubedb/apimachinery/client/scheme"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
)

const (
	gaTrackingCode = "UA-62096468-20"
)

func NewRootCmd(version string) *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:               "pg-operator",
		DisableAutoGenTag: true,
		PersistentPreRun: func(c *cobra.Command, args []string) {
			c.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Printf("FLAG: --%s=%q", flag.Name, flag.Value)
			})
			if opt.EnableAnalytics && gaTrackingCode != "" {
				if client, err := ga.NewClient(gaTrackingCode); err == nil {
					client.ClientID(opt.AnalyticsClientID)
					parts := strings.Split(c.CommandPath(), " ")
					client.Send(ga.NewEvent(parts[0], strings.Join(parts[1:], "/")).Label(version))
				}
			}
			scheme.AddToScheme(clientsetscheme.Scheme)
			opt.LoggerOptions = golog.ParseFlags(c.Flags())
		},
	}
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	// ref: https://github.com/kubernetes/kubernetes/issues/17162#issuecomment-225596212
	flag.CommandLine.Parse([]string{})
	rootCmd.PersistentFlags().BoolVar(&opt.EnableAnalytics, "analytics", opt.EnableAnalytics, "Send analytical events to Google Analytics")

	rootCmd.AddCommand(v.NewCmdVersion())
	rootCmd.AddCommand(NewCmdRun(version))
	rootCmd.AddCommand(NewCmdLeaderElection())

	return rootCmd
}
