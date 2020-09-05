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

package framework

import (
	"context"

	"github.com/appscode/go/crypto/rand"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbac_util "kmodules.xyz/client-go/rbac/v1"
)

const (
	statefulsets        = "statefulsets"
	pods                = "pods"
	configmaps          = "configmaps"
	podsecuritypolicies = "podsecuritypolicies"
	rbacApiGroup        = "rbac.authorization.k8s.io"
	GET                 = "get"
	LIST                = "list"
	PATCH               = "patch"
	CREATE              = "create"
	UPDATE              = "update"
	DELETE              = "delete"
	USE                 = "use"
	leaderLock          = "-leader-lock"
	APPS                = "apps"
	POLICY              = "policy"
	Role                = "Role"
	ServiceAccount      = "ServiceAccount"
)

func (i *Invocation) ServiceAccount() *core.ServiceAccount {
	return &core.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-pg"),
			Namespace: i.namespace,
		},
	}
}

func (i *Invocation) RoleForPostgres(meta metav1.ObjectMeta) *rbac.Role {
	return &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-pg"),
			Namespace: i.namespace,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{
					APPS,
				},
				ResourceNames: []string{
					meta.Name,
				},
				Resources: []string{
					statefulsets,
				},
				Verbs: []string{
					GET,
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					pods,
				},
				Verbs: []string{
					LIST,
					PATCH,
				},
			},
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					configmaps,
				},
				Verbs: []string{
					CREATE,
				},
			},
			{
				APIGroups: []string{
					"",
				},
				ResourceNames: []string{
					meta.Name + leaderLock,
				},
				Resources: []string{
					configmaps,
				},
				Verbs: []string{
					GET,
					UPDATE,
				},
			},
			{
				APIGroups: []string{
					POLICY,
				},
				ResourceNames: []string{
					meta.Name,
				},
				Resources: []string{
					podsecuritypolicies,
				},
				Verbs: []string{
					USE,
				},
			},
		},
	}
}

func (i *Invocation) RoleForSnapshot(meta metav1.ObjectMeta) *rbac.Role {
	return &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-pg"),
			Namespace: i.namespace,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{
					POLICY,
				},
				ResourceNames: []string{
					meta.Name,
				},
				Resources: []string{
					podsecuritypolicies,
				},
				Verbs: []string{
					USE,
				},
			},
		},
	}
}

func (i *Invocation) RoleBinding(saName string, roleName string) *rbac.RoleBinding {
	return &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-pg"),
			Namespace: i.namespace,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbacApiGroup,
			Kind:     Role,
			Name:     roleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      ServiceAccount,
				Namespace: i.namespace,
				Name:      saName,
			},
		},
	}
}

func (f *Framework) CreateServiceAccount(obj *core.ServiceAccount) error {
	_, err := f.kubeClient.CoreV1().ServiceAccounts(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) CreateRole(obj *rbac.Role) error {
	_, err := f.kubeClient.RbacV1().Roles(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) CreateRoleBinding(obj *rbac.RoleBinding) error {
	_, err := f.kubeClient.RbacV1().RoleBindings(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) DeleteRoleBinding(obj *rbac.RoleBinding) error {
	if err := f.kubeClient.RbacV1().RoleBindings(obj.Namespace).Delete(context.TODO(), obj.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}
	return rbac_util.WaitUntillRoleBindingDeleted(context.TODO(), f.kubeClient, obj.ObjectMeta)
}
