package framework

import (
	"github.com/appscode/go/crypto/rand"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/client-go/rbac/v1beta1"
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
	_, err := f.kubeClient.CoreV1().ServiceAccounts(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) CreateRole(obj *rbac.Role) error {
	_, err := f.kubeClient.RbacV1().Roles(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) CreateRoleBinding(obj *rbac.RoleBinding) error {
	_, err := f.kubeClient.RbacV1().RoleBindings(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) DeleteRoleBinding(obj *rbac.RoleBinding) error {
	err := f.kubeClient.RbacV1().RoleBindings(obj.Namespace).Delete(obj.Name, &metav1.DeleteOptions{})
	err = v1beta1.WaitUntillRoleBindingDeleted(f.kubeClient, obj.ObjectMeta)
	return err
}
