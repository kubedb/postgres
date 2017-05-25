package cmd

import (
	"fmt"

	"github.com/appscode/log"
	pcm "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1alpha1"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	"github.com/k8sdb/postgres/pkg/controller"
	"github.com/spf13/cobra"
	cgcmd "k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/util/runtime"
)

const (
	// Default tag
	canary = "canary-util"
)

func NewCmdRun() *cobra.Command {
	var (
		masterURL      string
		kubeconfigPath string
	)

	opt := &controller.Option{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run Postgres in Kubernetes",
		Run: func(cmd *cobra.Command, args []string) {
			config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				log.Fatalf("Could not get kubernetes config: %s", err)
			}

			// Check postgres docker image tag
			if err := amc.CheckDockerImageVersion(controller.ImagePostgres, opt.PostgresUtilTag); err != nil {
				log.Fatalf(`Image %v:%v not found.`, controller.ImagePostgres, opt.PostgresUtilTag)
			}

			// Check exporter docker image tag
			if err := amc.CheckDockerImageVersion(controller.ImageExporter, opt.ExporterTag); err != nil {
				log.Fatalf(`Image %v:%v not found.`, controller.ImageExporter, opt.ExporterTag)
			}

			client := clientset.NewForConfigOrDie(config)
			extClient := tcs.NewExtensionsForConfigOrDie(config)

			cgConfig, err := cgcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				log.Fatalf("Could not get kubernetes config: %s", err)
			}

			promClient, err := pcm.NewForConfig(cgConfig)
			if err != nil {
				log.Fatalln(err)
			}

			w := controller.New(client, extClient, promClient, opt)
			defer runtime.HandleCrash()
			fmt.Println("Starting operator...")
			w.RunAndHold()
		},
	}
	cmd.Flags().StringVar(&masterURL, "master", "", "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	cmd.Flags().StringVar(&opt.PostgresUtilTag, "postgres-util", canary, "Tag of postgres util")
	cmd.Flags().StringVar(&opt.ExporterNamespace, "exporter-ns", namespace(), "Namespace for monitoring exporter")
	cmd.Flags().StringVar(&opt.ExporterTag, "exporter", canary, "Tag of monitoring expoter")
	cmd.Flags().StringVar(&opt.GoverningService, "governing-service", "kubedb", "Governing service for database statefulset")
	return cmd
}

func namespace() string {
	if ns := os.Getenv("OPERATOR_NAMESPACE"); ns != "" {
		return ns
	}
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}
	return kapi.NamespaceDefault
}
