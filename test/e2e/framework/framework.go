package framework

import (
	"path/filepath"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"gomodules.xyz/cert/certstore"
	crd_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ka "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	appcat_cs "kmodules.xyz/custom-resources/client/clientset/versioned/typed/appcatalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"
	scs "stash.appscode.dev/stash/client/clientset/versioned"
)

var (
	DockerRegistry     = "kubedbci"
	SelfHostedOperator = false
	DBCatalogName      = "10.2-v4"
)

type Framework struct {
	restConfig       *rest.Config
	kubeClient       kubernetes.Interface
	apiExtKubeClient crd_cs.ApiextensionsV1beta1Interface
	dbClient         cs.Interface
	kaClient         ka.Interface
	appCatalogClient appcat_cs.AppcatalogV1alpha1Interface
	stashClient      scs.Interface
	namespace        string
	name             string
	StorageClass     string
	CertStore        *certstore.CertStore
}

func New(
	restConfig *rest.Config,
	kubeClient kubernetes.Interface,
	apiExtKubeClient crd_cs.ApiextensionsV1beta1Interface,
	dbClient cs.Interface,
	kaClient ka.Interface,
	appCatalogClient appcat_cs.AppcatalogV1alpha1Interface,
	stashClient scs.Interface,
	storageClass string,
) *Framework {
	store, err := certstore.NewCertStore(afero.NewMemMapFs(), filepath.Join("", "pki"))
	Expect(err).NotTo(HaveOccurred())

	err = store.InitCA()
	Expect(err).NotTo(HaveOccurred())
	return &Framework{
		restConfig:       restConfig,
		kubeClient:       kubeClient,
		apiExtKubeClient: apiExtKubeClient,
		dbClient:         dbClient,
		kaClient:         kaClient,
		appCatalogClient: appCatalogClient,
		stashClient:      stashClient,
		name:             "postgres-operator",
		namespace:        rand.WithUniqSuffix(api.ResourceSingularPostgres),
		StorageClass:     storageClass,
		CertStore:        store,
	}
}

func (f *Framework) Invoke() *Invocation {
	return &Invocation{
		Framework: f,
		app:       rand.WithUniqSuffix("postgres-e2e"),
	}
}

func (fi *Invocation) App() string {
	return fi.app
}

func (fi *Invocation) ExtClient() cs.Interface {
	return fi.dbClient
}

type Invocation struct {
	*Framework
	app string
}
