/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package matcher

import (
	"errors"
	"fmt"

	"github.com/onsi/gomega/types"
)

func MoreThan(expected int) types.GomegaMatcher {
	return &countMatcher{
		expected: expected,
	}
}

type countMatcher struct {
	expected int
}

func (matcher *countMatcher) Match(actual interface{}) (success bool, err error) {
	var total int
	if val, ok := actual.(int); ok {
		total = val
	} else if val, ok := actual.(int64); ok {
		// the interface can be int64 as well
		total = int(val)
	} else {
		return false, errors.New("match is neither int nor int64")
	}
	return total >= matcher.expected, nil
}

func (matcher *countMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to have snapshot more than %v", matcher.expected)
}

func (matcher *countMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected to have snapshot more than %v", matcher.expected)
}
