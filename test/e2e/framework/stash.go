/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package framework

import (
	"context"
	"fmt"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/pkg/controller"

	. "github.com/onsi/gomega"
	"gomodules.xyz/version"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
	v1alpha13 "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	"stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	stashV1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	"stash.appscode.dev/apimachinery/apis/stash/v1beta1"
	v1beta1_util "stash.appscode.dev/apimachinery/client/clientset/versioned/typed/stash/v1beta1/util"
)

func (f *Framework) FoundStashCRDs() bool {
	return controller.FoundStashCRDs(f.apiExtKubeClient)
}

func (i *Invocation) BackupConfiguration(dbMeta metav1.ObjectMeta, repo *stashV1alpha1.Repository) *v1beta1.BackupConfiguration {
	return &v1beta1.BackupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbMeta.Name + "-stash",
			Namespace: i.namespace,
		},
		Spec: v1beta1.BackupConfigurationSpec{
			Repository: core.LocalObjectReference{
				Name: repo.Name,
			},
			RetentionPolicy: v1alpha1.RetentionPolicy{
				KeepLast: 5,
				Prune:    true,
			},
			Schedule: "*/1 * * * *",
			BackupConfigurationTemplateSpec: v1beta1.BackupConfigurationTemplateSpec{
				Task: v1beta1.TaskRef{
					Name: i.getStashPGBackupTaskName(),
				},
				Target: &v1beta1.BackupTarget{
					Ref: v1beta1.TargetRef{
						APIVersion: v1alpha13.SchemeGroupVersion.String(),
						Kind:       v1alpha13.ResourceKindApp,
						Name:       dbMeta.Name,
					},
				},
			},
		},
	}
}

func (f *Framework) CreateBackupConfiguration(backupCfg *v1beta1.BackupConfiguration) error {
	_, err := f.stashClient.StashV1beta1().BackupConfigurations(backupCfg.Namespace).Create(context.TODO(), backupCfg, metav1.CreateOptions{})
	return err
}

func (f *Framework) DeleteBackupConfiguration(meta metav1.ObjectMeta) error {
	return f.stashClient.StashV1beta1().BackupConfigurations(meta.Namespace).Delete(context.TODO(), meta.Name, metav1.DeleteOptions{})
}

func (f *Framework) PauseBackupConfiguration(meta metav1.ObjectMeta) error {
	_, err := v1beta1_util.TryUpdateBackupConfiguration(context.TODO(), f.stashClient.StashV1beta1(), meta, func(in *v1beta1.BackupConfiguration) *v1beta1.BackupConfiguration {
		in.Spec.Paused = true
		return in
	}, metav1.UpdateOptions{})
	return err
}

func (f *Framework) Repository(dbMeta metav1.ObjectMeta) *stashV1alpha1.Repository {
	return &stashV1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbMeta.Name + "-stash",
			Namespace: f.namespace,
		},
		Spec: stashV1alpha1.RepositorySpec{
			WipeOut: true,
		},
	}
}

func (f *Framework) CreateRepository(repo *stashV1alpha1.Repository) error {
	_, err := f.stashClient.StashV1alpha1().Repositories(repo.Namespace).Create(context.TODO(), repo, metav1.CreateOptions{})

	return err
}

func (f *Framework) DeleteRepository(meta metav1.ObjectMeta) error {
	err := f.stashClient.StashV1alpha1().Repositories(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
	return err
}

func (f *Framework) EventuallySnapshotInRepository(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() int64 {
			repository, err := f.stashClient.StashV1alpha1().Repositories(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			return repository.Status.SnapshotCount
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (i *Invocation) RestoreSession(dbMeta metav1.ObjectMeta, repo *stashV1alpha1.Repository) *v1beta1.RestoreSession {
	return &v1beta1.RestoreSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbMeta.Name + "-stash",
			Namespace: i.namespace,
			Labels: map[string]string{
				"app":                 i.app,
				api.LabelDatabaseKind: api.ResourceKindPostgres,
			},
		},
		Spec: v1beta1.RestoreSessionSpec{
			Task: v1beta1.TaskRef{
				Name: i.getStashPGRestoreTaskName(),
			},
			Repository: core.LocalObjectReference{
				Name: repo.Name,
			},
			Rules: []v1beta1.Rule{
				{
					Snapshots: []string{"latest"},
				},
			},
			Target: &v1beta1.RestoreTarget{
				Ref: v1beta1.TargetRef{
					APIVersion: v1alpha13.SchemeGroupVersion.String(),
					Kind:       v1alpha13.ResourceKindApp,
					Name:       dbMeta.Name,
				},
			},
		},
	}
}

func (f *Framework) CreateRestoreSession(restoreSession *v1beta1.RestoreSession) error {
	_, err := f.stashClient.StashV1beta1().RestoreSessions(restoreSession.Namespace).Create(context.TODO(), restoreSession, metav1.CreateOptions{})
	return err
}

func (f Framework) DeleteRestoreSession(meta metav1.ObjectMeta) error {
	err := f.stashClient.StashV1beta1().RestoreSessions(meta.Namespace).Delete(context.TODO(), meta.Name, metav1.DeleteOptions{})
	return err
}

func (f *Framework) EventuallyRestoreSessionPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() v1beta1.RestoreSessionPhase {
		restoreSession, err := f.stashClient.StashV1beta1().RestoreSessions(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		if restoreSession.Status.Phase == v1beta1.RestoreSessionFailed {
			fmt.Println("Restoresession failed. ", restoreSession.Status.Stats)
		}
		return restoreSession.Status.Phase
	},
		time.Minute*7,
		time.Second*7,
	)
}

func (f *Framework) getStashPGBackupTaskName() string {
	pgVersion, err := f.dbClient.CatalogV1alpha1().PostgresVersions().Get(context.TODO(), DBCatalogName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	sv, err := version.NewVersion(pgVersion.Spec.Version)
	Expect(err).NotTo(HaveOccurred())

	return "postgres-backup-" + fmt.Sprintf("%v.%v", sv.Major(), sv.Minor())
}

func (f *Framework) getStashPGRestoreTaskName() string {
	pgVersion, err := f.dbClient.CatalogV1alpha1().PostgresVersions().Get(context.TODO(), DBCatalogName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	sv, err := version.NewVersion(pgVersion.Spec.Version)
	Expect(err).NotTo(HaveOccurred())

	return "postgres-restore-" + fmt.Sprintf("%v.%v", sv.Major(), sv.Minor())
}
