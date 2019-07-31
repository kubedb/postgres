module kubedb.dev/postgres

go 1.12

require (
	github.com/appscode/go v0.0.0-20190722173419-e454bf744023
	github.com/codeskyblue/go-sh v0.0.0-20190412065543-76bd3d59ff27
	github.com/coreos/prometheus-operator v0.30.0
	github.com/go-xorm/xorm v0.7.4
	github.com/lib/pq v1.1.0
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	gomodules.xyz/cert v1.0.0
	gomodules.xyz/stow v0.2.0
	k8s.io/api v0.0.0-20190503110853-61630f889b3c
	k8s.io/apiextensions-apiserver v0.0.0-20190516231611-bf6753f2aa24
	k8s.io/apimachinery v0.0.0-20190508063446-a3da69d3723c
	k8s.io/apiserver v0.0.0-20190516230822-f89599b3f645
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/component-base v0.0.0-20190508223741-40efa6d42997 // indirect
	k8s.io/kube-aggregator v0.0.0-20190508224022-f9852b6d3a84
	kmodules.xyz/client-go v0.0.0-20190715080709-7162a6c90b04
	kmodules.xyz/custom-resources v0.0.0-20190730174012-d0224972f055
	kmodules.xyz/monitoring-agent-api v0.0.0-20190513065523-186af167f817
	kmodules.xyz/objectstore-api v0.0.0-20190718002052-da668b440b0b
	kmodules.xyz/offshoot-api v0.0.0-20190715115723-36c8fce142c1
	kmodules.xyz/webhook-runtime v0.0.0-20190715115250-a84fbf77dd30
	kubedb.dev/apimachinery v0.0.0-20190731225000-6539c67a4a28
	stash.appscode.dev/stash v0.0.0-20190730144328-4ec6caf83810
	xorm.io/core v0.6.3
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest/autorest v0.5.0
	k8s.io/api => k8s.io/api v0.0.0-20190313235455-40a48860b5ab
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190315093550-53c4693659ed
	k8s.io/apimachinery => github.com/kmodules/apimachinery v0.0.0-20190508045248-a52a97a7a2bf
	k8s.io/apiserver => github.com/kmodules/apiserver v0.0.0-20190508082252-8397d761d4b5
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190314001948-2899ed30580f
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190314002645-c892ea32361a
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190314000054-4a91899592f4
	k8s.io/klog => k8s.io/klog v0.3.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190314000639-da8327669ac5
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190228160746-b3a7cee44a30
	k8s.io/metrics => k8s.io/metrics v0.0.0-20190314001731-1bd6a4002213
	k8s.io/utils => k8s.io/utils v0.0.0-20190221042446-c2654d5206da
)
