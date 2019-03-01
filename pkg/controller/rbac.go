package controller

import (
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	le "github.com/kubedb/postgres/pkg/leader_election"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	policy_v1beta1 "k8s.io/api/policy/v1beta1"
	rbac "k8s.io/api/rbac/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
	core_util "kmodules.xyz/client-go/core/v1"
	policy_util "kmodules.xyz/client-go/policy/v1beta1"
	rbac_util "kmodules.xyz/client-go/rbac/v1beta1"
)

func (c *Controller) ensureRole(postgres *api.Postgres) error {
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, postgres)
	if rerr != nil {
		return rerr
	}

	// Create new Roles
	_, _, err := rbac_util.CreateOrPatchRole(
		c.Client,
		metav1.ObjectMeta{
			Name:      postgres.OffshootName(),
			Namespace: postgres.Namespace,
		},
		func(in *rbac.Role) *rbac.Role {
			core_util.EnsureOwnerReference(&in.ObjectMeta, ref)
			in.Rules = []rbac.PolicyRule{
				{
					APIGroups:     []string{policy_v1beta1.GroupName},
					Resources:     []string{"podsecuritypolicies"},
					Verbs:         []string{"use"},
					ResourceNames: []string{postgres.OffshootName()},
				},
				{
					APIGroups:     []string{apps.GroupName},
					Resources:     []string{"statefulsets"},
					Verbs:         []string{"get"},
					ResourceNames: []string{postgres.OffshootName()},
				},
				{
					APIGroups: []string{core.GroupName},
					Resources: []string{"pods"},
					Verbs:     []string{"list", "patch"},
				},
				{
					APIGroups: []string{core.GroupName},
					Resources: []string{"configmaps"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{core.GroupName},
					Resources:     []string{"configmaps"},
					Verbs:         []string{"get", "update"},
					ResourceNames: []string{le.GetLeaderLockName(postgres.OffshootName())},
				},
			}
			return in
		},
	)
	return err
}

func (c *Controller) ensureSnapshotRole(postgres *api.Postgres) error {
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, postgres)
	if rerr != nil {
		return rerr
	}
	// Create new Roles
	_, _, err := rbac_util.CreateOrPatchRole(
		c.Client,
		metav1.ObjectMeta{
			Name:      postgres.SnapshotSAName(),
			Namespace: postgres.Namespace,
		},
		func(in *rbac.Role) *rbac.Role {
			core_util.EnsureOwnerReference(&in.ObjectMeta, ref)
			in.Rules = []rbac.PolicyRule{
				{
					APIGroups:     []string{policy_v1beta1.GroupName},
					Resources:     []string{"podsecuritypolicies"},
					Verbs:         []string{"use"},
					ResourceNames: []string{postgres.SnapshotSAName()},
				},
			}
			return in
		},
	)
	return err
}

func (c *Controller) createServiceAccount(postgres *api.Postgres, saName string) error {
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, postgres)
	if rerr != nil {
		return rerr
	}
	// Create new ServiceAccount
	_, _, err := core_util.CreateOrPatchServiceAccount(
		c.Client,
		metav1.ObjectMeta{
			Name:      saName,
			Namespace: postgres.Namespace,
		},
		func(in *core.ServiceAccount) *core.ServiceAccount {
			core_util.EnsureOwnerReference(&in.ObjectMeta, ref)
			return in
		},
	)
	return err
}

func (c *Controller) createRoleBinding(postgres *api.Postgres) error {
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, postgres)
	if rerr != nil {
		return rerr
	}
	// Ensure new RoleBindings
	_, _, err := rbac_util.CreateOrPatchRoleBinding(
		c.Client,
		metav1.ObjectMeta{
			Name:      postgres.OffshootName(),
			Namespace: postgres.Namespace,
		},
		func(in *rbac.RoleBinding) *rbac.RoleBinding {
			core_util.EnsureOwnerReference(&in.ObjectMeta, ref)
			in.RoleRef = rbac.RoleRef{
				APIGroup: rbac.GroupName,
				Kind:     "Role",
				Name:     postgres.OffshootName(),
			}
			in.Subjects = []rbac.Subject{
				{
					Kind:      rbac.ServiceAccountKind,
					Name:      postgres.OffshootName(),
					Namespace: postgres.Namespace,
				},
			}
			return in
		},
	)
	return err
}

func (c *Controller) createSnapshotRoleBinding(postgres *api.Postgres) error {
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, postgres)
	if rerr != nil {
		return rerr
	}
	// Ensure new RoleBindings
	_, _, err := rbac_util.CreateOrPatchRoleBinding(
		c.Client,
		metav1.ObjectMeta{
			Name:      postgres.SnapshotSAName(),
			Namespace: postgres.Namespace,
		},
		func(in *rbac.RoleBinding) *rbac.RoleBinding {
			core_util.EnsureOwnerReference(&in.ObjectMeta, ref)
			in.RoleRef = rbac.RoleRef{
				APIGroup: rbac.GroupName,
				Kind:     "Role",
				Name:     postgres.SnapshotSAName(),
			}
			in.Subjects = []rbac.Subject{
				{
					Kind:      rbac.ServiceAccountKind,
					Name:      postgres.SnapshotSAName(),
					Namespace: postgres.Namespace,
				},
			}
			return in
		},
	)
	return err
}

func (c *Controller) ensurePSP(postgres *api.Postgres) error {
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, postgres)
	if rerr != nil {
		return rerr
	}

	noEscalation := false
	_, _, err := policy_util.CreateOrPatchPodSecurityPolicy(c.Client,
		metav1.ObjectMeta{
			Name: postgres.OffshootName(),
		},
		func(in *policy_v1beta1.PodSecurityPolicy) *policy_v1beta1.PodSecurityPolicy {
			//TODO: possible function EnsureOwnerReference(&psp.ObjectMeta, ref) in kutil for non namespaced resources.
			in.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: ref.APIVersion,
					Kind:       ref.Kind,
					Name:       ref.Name,
					UID:        ref.UID,
				},
			}
			in.Spec = policy_v1beta1.PodSecurityPolicySpec{
				Privileged:               false,
				AllowPrivilegeEscalation: &noEscalation,
				Volumes: []policy_v1beta1.FSType{
					policy_v1beta1.All,
				},
				HostIPC:     false,
				HostNetwork: false,
				HostPID:     false,
				RunAsUser: policy_v1beta1.RunAsUserStrategyOptions{
					Rule: policy_v1beta1.RunAsUserStrategyRunAsAny,
				},
				SELinux: policy_v1beta1.SELinuxStrategyOptions{
					Rule: policy_v1beta1.SELinuxStrategyRunAsAny,
				},
				FSGroup: policy_v1beta1.FSGroupStrategyOptions{
					Rule: policy_v1beta1.FSGroupStrategyRunAsAny,
				},
				SupplementalGroups: policy_v1beta1.SupplementalGroupsStrategyOptions{
					Rule: policy_v1beta1.SupplementalGroupsStrategyRunAsAny,
				},
				AllowedCapabilities: []core.Capability{
					"IPC_LOCK",
					"SYS_RESOURCE",
				},
			}
			return in
		},
	)
	return err
}

func (c *Controller) ensureSnapshotPSP(postgres *api.Postgres) error {
	ref, rerr := reference.GetReference(clientsetscheme.Scheme, postgres)
	if rerr != nil {
		return rerr
	}

	noEscalation := false
	_, _, err := policy_util.CreateOrPatchPodSecurityPolicy(c.Client,
		metav1.ObjectMeta{
			Name: postgres.SnapshotSAName(),
		},
		func(in *policy_v1beta1.PodSecurityPolicy) *policy_v1beta1.PodSecurityPolicy {
			//TODO: possible function EnsureOwnerReference(&psp.ObjectMeta, ref) in kutil for non namespaced resources.
			in.OwnerReferences = []metav1.OwnerReference{
				{
					APIVersion: ref.APIVersion,
					Kind:       ref.Kind,
					Name:       ref.Name,
					UID:        ref.UID,
				},
			}
			in.Spec = policy_v1beta1.PodSecurityPolicySpec{
				Privileged:               false,
				AllowPrivilegeEscalation: &noEscalation,
				Volumes: []policy_v1beta1.FSType{
					policy_v1beta1.All,
				},
				HostIPC:     false,
				HostNetwork: false,
				HostPID:     false,
				RunAsUser: policy_v1beta1.RunAsUserStrategyOptions{
					Rule: policy_v1beta1.RunAsUserStrategyRunAsAny,
				},
				SELinux: policy_v1beta1.SELinuxStrategyOptions{
					Rule: policy_v1beta1.SELinuxStrategyRunAsAny,
				},
				FSGroup: policy_v1beta1.FSGroupStrategyOptions{
					Rule: policy_v1beta1.FSGroupStrategyRunAsAny,
				},
				SupplementalGroups: policy_v1beta1.SupplementalGroupsStrategyOptions{
					Rule: policy_v1beta1.SupplementalGroupsStrategyRunAsAny,
				},
			}
			return in
		},
	)
	return err
}

func (c *Controller) ensureRBACStuff(postgres *api.Postgres) error {
	//Create PSP
	if err := c.ensurePSP(postgres); err != nil {
		return err
	}

	// Create New Role
	if err := c.ensureRole(postgres); err != nil {
		return err
	}

	// Create New ServiceAccount
	if err := c.createServiceAccount(postgres, postgres.OffshootName()); err != nil {
		if !kerr.IsAlreadyExists(err) {
			return err
		}
	}

	// Create New RoleBinding
	if err := c.createRoleBinding(postgres); err != nil {
		return err
	}
	//Create PSP for snapshot
	if err := c.ensureSnapshotPSP(postgres); err != nil {
		return err
	}

	//Role for snapshot
	if err := c.ensureSnapshotRole(postgres); err != nil {
		return err
	}

	// ServiceAccount for snapshot
	if err := c.createServiceAccount(postgres, postgres.SnapshotSAName()); err != nil {
		if !kerr.IsAlreadyExists(err) {
			return err
		}
	}

	// Create New RoleBinding for snapshot
	if err := c.createSnapshotRoleBinding(postgres); err != nil {
		return err
	}

	return nil
}
