package e2e_test

import (
	"flag"
	"path/filepath"
	"testing"
	"time"

	"github.com/appscode/go/log"
	logs "github.com/appscode/go/log/golog"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/typed/kubedb/v1alpha1"
	snapc "github.com/kubedb/apimachinery/pkg/controller/snapshot"
	"github.com/kubedb/postgres/pkg/controller"
	"github.com/kubedb/postgres/pkg/docker"
	"github.com/kubedb/postgres/test/e2e/framework"
	"github.com/mitchellh/go-homedir"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	crd_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	storageClass string
	registry     string
	enableRbac   bool
)

func init() {
	flag.StringVar(&storageClass, "storageclass", "", "Kubernetes StorageClass name")
	flag.StringVar(&registry, "docker-registry", "kubedb", "User provided docker repository")
	flag.BoolVar(&enableRbac, "rbac", false, "Enable RBAC for database workloads")
}

const (
	TIMEOUT = 20 * time.Minute
)

var (
	ctrl *controller.Controller
	root *framework.Framework
)

func TestE2e(t *testing.T) {
	logs.InitLogs()
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(TIMEOUT)

	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "e2e Suite", []Reporter{junitReporter})
}

var _ = BeforeSuite(func() {

	userHome, err := homedir.Dir()
	Expect(err).NotTo(HaveOccurred())

	// Kubernetes config
	kubeconfigPath := filepath.Join(userHome, ".kube/config")
	By("Using kubeconfig from " + kubeconfigPath)
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	Expect(err).NotTo(HaveOccurred())
	// Clients
	kubeClient := kubernetes.NewForConfigOrDie(config)
	apiExtKubeClient := crd_cs.NewForConfigOrDie(config)
	extClient := cs.NewForConfigOrDie(config)
	// Framework
	root = framework.New(config, kubeClient, extClient, storageClass)

	By("Using namespace " + root.Namespace())

	// Create namespace
	err = root.CreateNamespace()
	Expect(err).NotTo(HaveOccurred())

	cronController := snapc.NewCronController(kubeClient, extClient)
	// Start Cron
	cronController.StartCron()

	opt := controller.Options{
		Docker: docker.Docker{
			Registry: registry,
		},
		OperatorNamespace: root.Namespace(),
		GoverningService:  api.DatabaseNamePrefix,
		MaxNumRequeues:    5,
		EnableRbac:        enableRbac,
		AnalyticsClientID: "$kubedb$postgres$e2e",
	}

	// Controller
	ctrl = controller.New(kubeClient, apiExtKubeClient, extClient, nil, cronController, opt)
	err = ctrl.Setup()
	if err != nil {
		log.Fatalln(err)
	}
	ctrl.Run()
})

var _ = AfterSuite(func() {
	root.CleanPostgres()
	root.CleanDormantDatabase()
	root.CleanSnapshot()
	err := root.DeleteNamespace()
	Expect(err).NotTo(HaveOccurred())
	By("Deleted namespace")
})
