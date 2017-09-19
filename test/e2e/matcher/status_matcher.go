package matcher

import (
	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/onsi/gomega/types"
)

func HavePaused() types.GomegaMatcher {
	return &statusMatcher{
		expected: tapi.DormantDatabasePhasePaused,
	}
}

func HaveWipedOut() types.GomegaMatcher {
	return &statusMatcher{
		expected: tapi.DormantDatabasePhaseWipedOut,
	}
}

type statusMatcher struct {
	expected tapi.DormantDatabasePhase
}

func (matcher *statusMatcher) Match(actual interface{}) (success bool, err error) {
	phase := actual.(tapi.DormantDatabasePhase)
	return phase == matcher.expected, nil
}

func (matcher *statusMatcher) FailureMessage(actual interface{}) (message string) {
	return "Expected to be Running all Pods"
}

func (matcher *statusMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return "Expected to be not Running all Pods"
}
