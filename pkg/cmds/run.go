package cmds

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/appscode/go/log"
	pcm "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1alpha1"
	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	tcs "github.com/k8sdb/apimachinery/client/typed/kubedb/v1alpha1"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	"github.com/k8sdb/apimachinery/pkg/migrator"
	"github.com/k8sdb/postgres/pkg/controller"
	"github.com/spf13/cobra"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func NewCmdRun() *cobra.Command {
	var (
		masterURL      string
		kubeconfigPath string
	)

	opt := controller.Options{
		OperatorNamespace: namespace(),
		ExporterTag:       "0.6.0",
		GoverningService:  "kubedb",
		Address:           ":8080",
		EnableRbac:        false,
	}

	cmd := &cobra.Command{
		Use:               "run",
		Short:             "Run Postgres in Kubernetes",
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {
			config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				log.Fatalf("Could not get kubernetes config: %s", err)
			}

			client := clientset.NewForConfigOrDie(config)
			apiExtKubeClient := apiextensionsclient.NewForConfigOrDie(config)
			extClient := tcs.NewForConfigOrDie(config)
			promClient, err := pcm.NewForConfig(config)
			if err != nil {
				log.Fatalln(err)
			}

			cronController := amc.NewCronController(client, extClient)
			// Start Cron
			cronController.StartCron()
			// Stop Cron
			defer cronController.StopCron()

			w := controller.New(client, apiExtKubeClient, extClient, promClient, cronController, opt)
			defer runtime.HandleCrash()

			tprMigrator := migrator.NewMigrator(client, apiExtKubeClient, extClient)
			err = tprMigrator.RunMigration(
				&tapi.Postgres{},
				&tapi.Snapshot{},
				&tapi.DormantDatabase{},
			)
			if err != nil {
				log.Fatalln(err)
			}

			fmt.Println("Starting operator...")
			w.RunAndHold()
		},
	}
	// operator flags
	cmd.Flags().StringVar(&masterURL, "master", masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	cmd.Flags().StringVar(&opt.GoverningService, "governing-service", opt.GoverningService, "Governing service for database statefulset")
	cmd.Flags().StringVar(&opt.ExporterTag, "exporter-tag", opt.ExporterTag, "Tag of kubedb/operator used as exporter")
	cmd.Flags().StringVar(&opt.Address, "address", opt.Address, "Address to listen on for web interface and telemetry.")
	cmd.Flags().BoolVar(&opt.EnableRbac, "rbac", opt.EnableRbac, "Enable RBAC for database workloads")

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
	return metav1.NamespaceDefault
}
