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
	"fmt"
	"os"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/postgres/pkg/controller"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1 "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/constants/aws"
	"kmodules.xyz/constants/azure"
	"kmodules.xyz/constants/google"
	"kmodules.xyz/constants/openstack"
	"stash.appscode.dev/apimachinery/pkg/restic"
)

var (
	CustomSecretSuffix = "custom-secret"
	CustomUsername     = "username1234567890"
	CustomPassword     = "password0987654321"
)

func (i *Invocation) SecretForLocalBackend() *core.Secret {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-local"),
			Namespace: i.namespace,
		},
		Data: map[string][]byte{},
	}
}

func (i *Invocation) SecretForS3Backend() *core.Secret {
	if os.Getenv(aws.AWS_ACCESS_KEY_ID) == "" ||
		os.Getenv(aws.AWS_SECRET_ACCESS_KEY) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-s3"),
			Namespace: i.namespace,
		},
		Data: map[string][]byte{
			aws.AWS_ACCESS_KEY_ID:     []byte(os.Getenv(aws.AWS_ACCESS_KEY_ID)),
			aws.AWS_SECRET_ACCESS_KEY: []byte(os.Getenv(aws.AWS_SECRET_ACCESS_KEY)),
		},
	}
}

func (i *Invocation) SecretForMinioBackend() *core.Secret {
	if os.Getenv(aws.AWS_ACCESS_KEY_ID) == "" ||
		os.Getenv(aws.AWS_SECRET_ACCESS_KEY) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-s3"),
			Namespace: i.namespace,
		},
		Data: map[string][]byte{
			aws.AWS_ACCESS_KEY_ID:     []byte(os.Getenv(aws.AWS_ACCESS_KEY_ID)),
			aws.AWS_SECRET_ACCESS_KEY: []byte(os.Getenv(aws.AWS_SECRET_ACCESS_KEY)),
			aws.CA_CERT_DATA:          i.CertStore.CACertBytes(),
		},
	}
}

func (i *Invocation) SecretForMinioServer() *core.Secret {

	if os.Getenv(aws.AWS_ACCESS_KEY_ID) == "" ||
		os.Getenv(aws.AWS_SECRET_ACCESS_KEY) == "" {
		return &core.Secret{}
	}

	crt, key, err := i.CertStore.NewServerCertPairBytes(i.MinioServerSANs())
	if err != nil {
		return nil
	}
	Expect(err).NotTo(HaveOccurred())

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-s3"),
			Namespace: i.namespace,
		},
		Data: map[string][]byte{
			aws.AWS_ACCESS_KEY_ID:     []byte(os.Getenv(aws.AWS_ACCESS_KEY_ID)),
			aws.AWS_SECRET_ACCESS_KEY: []byte(os.Getenv(aws.AWS_SECRET_ACCESS_KEY)),
			MINIO_PUBLIC_CRT_NAME:     []byte(string(crt) + "\n" + string(i.CertStore.CACertBytes())),
			MINIO_PRIVATE_KEY_NAME:    key,
		},
	}
}

func (i *Invocation) SecretForGCSBackend() *core.Secret {
	jsonKey := google.ServiceAccountFromEnv()
	if jsonKey == "" || os.Getenv(google.GOOGLE_PROJECT_ID) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-gcs"),
			Namespace: i.namespace,
		},
		Data: map[string][]byte{
			google.GOOGLE_PROJECT_ID:               []byte(os.Getenv(google.GOOGLE_PROJECT_ID)),
			google.GOOGLE_SERVICE_ACCOUNT_JSON_KEY: []byte(jsonKey),
		},
	}
}

func (i *Invocation) SecretForAzureBackend() *core.Secret {
	if os.Getenv(azure.AZURE_ACCOUNT_NAME) == "" ||
		os.Getenv(azure.AZURE_ACCOUNT_KEY) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-azure"),
			Namespace: i.namespace,
		},
		Data: map[string][]byte{
			azure.AZURE_ACCOUNT_NAME: []byte(os.Getenv(azure.AZURE_ACCOUNT_NAME)),
			azure.AZURE_ACCOUNT_KEY:  []byte(os.Getenv(azure.AZURE_ACCOUNT_KEY)),
		},
	}
}

func (i *Invocation) SecretForSwiftBackend() *core.Secret {
	if os.Getenv(openstack.OS_AUTH_URL) == "" ||
		(os.Getenv(openstack.OS_TENANT_ID) == "" && os.Getenv(openstack.OS_TENANT_NAME) == "") ||
		os.Getenv(openstack.OS_USERNAME) == "" ||
		os.Getenv(openstack.OS_PASSWORD) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-swift"),
			Namespace: i.namespace,
		},
		Data: map[string][]byte{
			openstack.OS_AUTH_URL:    []byte(os.Getenv(openstack.OS_AUTH_URL)),
			openstack.OS_TENANT_ID:   []byte(os.Getenv(openstack.OS_TENANT_ID)),
			openstack.OS_TENANT_NAME: []byte(os.Getenv(openstack.OS_TENANT_NAME)),
			openstack.OS_USERNAME:    []byte(os.Getenv(openstack.OS_USERNAME)),
			openstack.OS_PASSWORD:    []byte(os.Getenv(openstack.OS_PASSWORD)),
			openstack.OS_REGION_NAME: []byte(os.Getenv(openstack.OS_REGION_NAME)),
		},
	}
}

func (i *Invocation) SecretForDatabaseAuthentication(meta metav1.ObjectMeta) *core.Secret {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", meta.Name, CustomSecretSuffix),
			Namespace: meta.Namespace,
		},
		StringData: map[string]string{
			controller.PostgresUser:     CustomUsername,
			controller.PostgresPassword: CustomPassword,
		},
	}
}

func (i *Invocation) SecretForDatabaseAuthenticationWithLabel(meta metav1.ObjectMeta) *core.Secret {
	//this Label mimics a secret created and manged by kubedb and not user.
	// It should get deleted during wipeout
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("kubedb-%v-%v", meta.Name, CustomSecretSuffix),
			Namespace: meta.Namespace,
			Labels: map[string]string{
				meta_util.ManagedByLabelKey: api.GenericKey,
			},
		},
		StringData: map[string]string{
			controller.PostgresUser:     CustomUsername,
			controller.PostgresPassword: CustomPassword,
		},
	}
}

func (i *Invocation) PatchSecretForRestic(secret *core.Secret) *core.Secret {
	if secret == nil {
		return secret
	}

	secret.StringData = v1.UpsertMap(secret.StringData, map[string]string{
		restic.RESTIC_PASSWORD: "RESTIC_PASSWORD",
	})
	return secret
}

// TODO: Add more methods for Swift, Backblaze B2, Rest server backend.

func (f *Framework) CreateSecret(obj *core.Secret) error {
	_, err := f.kubeClient.CoreV1().Secrets(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) UpdateSecret(meta metav1.ObjectMeta, transformer func(core.Secret) core.Secret) error {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return nil
		} else if err == nil {
			modified := transformer(*cur)
			_, err = f.kubeClient.CoreV1().Secrets(cur.Namespace).Update(&modified)
			if err == nil {
				return nil
			}
		}
		log.Errorf("Attempt %d failed to update Secret %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(updateRetryInterval)
	}
	return fmt.Errorf("failed to update Secret %s@%s after %d attempts", meta.Name, meta.Namespace, attempt)
}

func (f *Framework) DeleteSecret(meta metav1.ObjectMeta) error {
	err := f.kubeClient.CoreV1().Secrets(meta.Namespace).Delete(meta.Name, deleteInForeground())
	if !kerr.IsNotFound(err) {
		return err
	}
	return nil
}

func (f *Framework) EventuallyDBSecretCount(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	labelMap := map[string]string{
		api.LabelDatabaseKind: api.ResourceKindPostgres,
		api.LabelDatabaseName: meta.Name,
	}
	labelSelector := labels.SelectorFromSet(labelMap)
	return Eventually(
		func() int {
			secretList, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).List(
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			Expect(err).NotTo(HaveOccurred())
			return len(secretList.Items)
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) CheckSecret(secret *core.Secret) error {
	_, err := f.kubeClient.CoreV1().Secrets(f.namespace).Get(secret.Name, metav1.GetOptions{})
	return err
}
