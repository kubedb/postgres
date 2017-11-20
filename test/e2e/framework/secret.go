package framework

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (fi *Invocation) SecretForLocalBackend() *core.Secret {
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-local"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{},
	}
}

func (fi *Invocation) SecretForS3Backend() *core.Secret {
	if os.Getenv(tapi.AWS_ACCESS_KEY_ID) == "" ||
		os.Getenv(tapi.AWS_SECRET_ACCESS_KEY) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-s3"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{
			tapi.AWS_ACCESS_KEY_ID:     []byte(os.Getenv(tapi.AWS_ACCESS_KEY_ID)),
			tapi.AWS_SECRET_ACCESS_KEY: []byte(os.Getenv(tapi.AWS_SECRET_ACCESS_KEY)),
		},
	}
}

func (fi *Invocation) SecretForGCSBackend() *core.Secret {
	if os.Getenv(tapi.GOOGLE_PROJECT_ID) == "" ||
		(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" && os.Getenv(tapi.GOOGLE_SERVICE_ACCOUNT_JSON_KEY) == "") {
		return &core.Secret{}
	}

	jsonKey := os.Getenv(tapi.GOOGLE_SERVICE_ACCOUNT_JSON_KEY)
	if jsonKey == "" {
		if keyBytes, err := ioutil.ReadFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")); err == nil {
			jsonKey = string(keyBytes)
		}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-gcs"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{
			tapi.GOOGLE_PROJECT_ID:               []byte(os.Getenv(tapi.GOOGLE_PROJECT_ID)),
			tapi.GOOGLE_SERVICE_ACCOUNT_JSON_KEY: []byte(jsonKey),
		},
	}
}

func (fi *Invocation) SecretForAzureBackend() *core.Secret {
	if os.Getenv(tapi.AZURE_ACCOUNT_NAME) == "" ||
		os.Getenv(tapi.AZURE_ACCOUNT_KEY) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-azure"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{
			tapi.AZURE_ACCOUNT_NAME: []byte(os.Getenv(tapi.AZURE_ACCOUNT_NAME)),
			tapi.AZURE_ACCOUNT_KEY:  []byte(os.Getenv(tapi.AZURE_ACCOUNT_KEY)),
		},
	}
}

func (fi *Invocation) SecretForSwiftBackend() *core.Secret {
	if os.Getenv(tapi.OS_AUTH_URL) == "" ||
		(os.Getenv(tapi.OS_TENANT_ID) == "" && os.Getenv(tapi.OS_TENANT_NAME) == "") ||
		os.Getenv(tapi.OS_USERNAME) == "" ||
		os.Getenv(tapi.OS_PASSWORD) == "" {
		return &core.Secret{}
	}

	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-swift"),
			Namespace: fi.namespace,
		},
		Data: map[string][]byte{
			tapi.OS_AUTH_URL:    []byte(os.Getenv(tapi.OS_AUTH_URL)),
			tapi.OS_TENANT_ID:   []byte(os.Getenv(tapi.OS_TENANT_ID)),
			tapi.OS_TENANT_NAME: []byte(os.Getenv(tapi.OS_TENANT_NAME)),
			tapi.OS_USERNAME:    []byte(os.Getenv(tapi.OS_USERNAME)),
			tapi.OS_PASSWORD:    []byte(os.Getenv(tapi.OS_PASSWORD)),
			tapi.OS_REGION_NAME: []byte(os.Getenv(tapi.OS_REGION_NAME)),
		},
	}
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
	return fmt.Errorf("Failed to update Secret %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}

func (f *Framework) DeleteSecret(meta metav1.ObjectMeta) error {
	return f.kubeClient.CoreV1().Secrets(meta.Namespace).Delete(meta.Name, deleteInForeground())
}
