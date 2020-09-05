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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/gomega"
	"gomodules.xyz/cert"
	"gomodules.xyz/stow"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	apps_util "kmodules.xyz/client-go/apps/v1"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/client-go/tools/portforward"
	v1 "kmodules.xyz/objectstore-api/api/v1"
	"kmodules.xyz/objectstore-api/osm"
)

const (
	MINIO_PUBLIC_CRT_NAME  = "public.crt"
	MINIO_PRIVATE_KEY_NAME = "private.key"

	MINIO_ACCESS_KEY = "MINIO_ACCESS_KEY"
	MINIO_SECRET_KEY = "MINIO_SECRET_KEY"

	AWS_ACCESS_KEY_ID     = "AWS_ACCESS_KEY_ID"
	AWS_SECRET_ACCESS_KEY = "AWS_SECRET_ACCESS_KEY"

	MINIO_CERTS_MOUNTPATH = "/root/.minio/certs"
	MinioSecretHTTP       = "minio-secret-http"
	MinioSecretHTTPS      = "minio-secret-https"
	MinioPVC              = "minio-pv-claim"
	MinioServiceHTTP      = "minio-service-http"
	MinioServiceHTTPS     = "minio-service-https"
	MinioServerHTTP       = "minio-http"
	MinioServerHTTPS      = "minio-https"
	PORT                  = 443
	S3_BUCKET_NAME        = "S3_BUCKET_NAME"
	minikubeIP            = "192.168.99.100"
	localIP               = "127.0.0.1"
)

var (
	mcred        *core.Secret
	mpvc         *core.PersistentVolumeClaim
	mdeploy      *apps.Deployment
	msrvc        core.Service
	MinioTLS     bool
	minioPodName = ""
	MinioService = ""
)

func (i *Invocation) CreateMinioServer(tls bool, ips []net.IP, minioBackendSecret *core.Secret) (string, error) {
	MinioTLS = tls
	//creating service for minio server
	var err error
	if MinioTLS {
		err = i.CreateHTTPSMinioServer(minioBackendSecret)
	} else {
		err = i.CreateHTTPMinioServer(minioBackendSecret)
	}
	if err != nil {
		return "", err
	}

	var endPoint string
	if tls {
		endPoint = "https://" + i.MinioServiceAddress()
	} else {
		endPoint = "http://" + i.MinioServiceAddress()
	}
	return endPoint, nil
}

func (i *Invocation) CreateHTTPMinioServer(minioBackendSecret *core.Secret) error {
	msrvc = i.ServiceForMinioServer()
	MinioService = MinioServiceHTTP
	_, err := i.CreateService(msrvc)
	if err != nil {
		return err
	}

	//creating secret for minio server
	mcred = i.SecretForS3Backend()
	mcred.Name = MinioSecretHTTP
	if err := i.CreateSecret(mcred); err != nil {
		return err
	}

	//creating pvc for minio server
	mpvc = i.GetPersistentVolumeClaim()
	mpvc.Name = MinioPVC + "http"
	mpvc.Labels = map[string]string{"app": "minio-storage-claim"}

	err = i.CreatePersistentVolumeClaim(mpvc)
	if err != nil {
		return err
	}
	//creating deployment for minio server
	mdeploy = i.MinioServerDeploymentHTTP()
	// if tls not enabled then don't mount secret for cacerts
	//mdeploy.Spec.Template.Spec.Containers = i.RemoveSecretVolumeMount(mdeploy.Spec.Template.Spec.Containers)
	deploy, err := i.CreateDeploymentForMinioServer(mdeploy)
	if err != nil {
		return err
	}
	if err = apps_util.WaitUntilDeploymentReady(context.TODO(), i.kubeClient, deploy.ObjectMeta); err != nil {
		return err
	}

	if err = i.CreateBucket(deploy, minioBackendSecret, false); err != nil {
		return err
	}
	return nil
}

func (i *Invocation) CreateHTTPSMinioServer(minioBackendSecret *core.Secret) error {
	msrvc = i.ServiceForMinioServer()
	MinioService = MinioServiceHTTPS
	_, err := i.CreateService(msrvc)
	if err != nil {
		return err
	}

	//creating secret with CA for minio server
	mcred = i.SecretForMinioServer()

	mcred.Name = MinioSecretHTTPS
	if err := i.CreateSecret(mcred); err != nil {
		return err
	}

	//creating pvc for minio server
	mpvc = i.GetPersistentVolumeClaim()
	mpvc.Name = MinioPVC + "https"
	mpvc.Labels = map[string]string{"app": "minio-storage-claim"}

	err = i.CreatePersistentVolumeClaim(mpvc)
	if err != nil {
		return nil
	}

	//creating deployment for minio server
	mdeploy = i.MinioServerDeploymentHTTPS(true)
	deploy, err := i.CreateDeploymentForMinioServer(mdeploy)
	if err != nil {
		return err
	}

	err = apps_util.WaitUntilDeploymentReady(context.TODO(), i.kubeClient, deploy.ObjectMeta)
	if err != nil {
		return err
	}

	err = i.CreateBucket(deploy, minioBackendSecret, true)
	if err != nil {
		return err
	}
	return nil
}

func (f *Framework) IsTLS() bool {
	return MinioTLS
}

func (f *Framework) IsMinio(backend *v1.Backend) bool {
	if backend == nil || backend.S3 == nil {
		return false
	}
	return backend.S3.Endpoint != "" && !strings.HasSuffix(backend.S3.Endpoint, ".amazonaws.com")
}

func (f *Framework) ForwardMinioPort(clientPodName string) (*portforward.Tunnel, error) {
	tunnel := portforward.NewTunnel(
		f.kubeClient.CoreV1().RESTClient(),
		f.restConfig,
		f.namespace,
		clientPodName,
		PORT,
	)
	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func (i *Invocation) CreateBucket(deployment *apps.Deployment, minioBackendSecret *core.Secret, tls bool) error {
	endPoint := ""
	podlist, err := i.kubeClient.CoreV1().Pods(deployment.ObjectMeta.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector)})
	if err != nil {
		return err
	}
	if len(podlist.Items) > 0 {
		for _, pod := range podlist.Items {
			minioPodName = pod.Name
			break
		}
	}

	tunnel, err := i.ForwardMinioPort(minioPodName)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	err = wait.PollImmediate(2*time.Second, 10*time.Minute, func() (bool, error) {
		if tls {
			endPoint = fmt.Sprintf("https://%s:%d", localIP, tunnel.Local)
		} else {
			endPoint = fmt.Sprintf("http://%s:%d", localIP, tunnel.Local)
		}
		err = i.CreateMinioBucket(os.Getenv(S3_BUCKET_NAME), minioBackendSecret, endPoint)
		if err != nil {
			return false, nil //dont return error
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (i *Invocation) CreateMinioBucket(bucketName string, minioBackendSecret *core.Secret, endPoint string) error {
	Storage := v1.Backend{
		StorageSecretName: minioBackendSecret.Name,
		S3: &v1.S3Spec{
			Bucket:   os.Getenv(S3_BUCKET_NAME),
			Endpoint: endPoint,
		},
	}
	cfg, err := osm.NewOSMContext(i.kubeClient, Storage, i.Namespace())
	if err != nil {
		return err
	}

	loc, err := stow.Dial(cfg.Provider, cfg.Config)
	if err != nil {
		return err
	}

	containerID, err := Storage.Container()
	if err != nil {
		return err
	}
	_, err = loc.CreateContainer(containerID)
	if err != nil {
		return err
	}

	return nil
}

func (i *Invocation) CreateDeploymentForMinioServer(obj *apps.Deployment) (*apps.Deployment, error) {
	newDeploy, err := i.kubeClient.AppsV1().Deployments(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return newDeploy, err
}

func (i *Invocation) MinioServerDeploymentHTTPS(tls bool) *apps.Deployment {
	labels := map[string]string{
		"app": MinioServerHTTPS,
	}
	CAvol := []core.Volume{
		{
			Name: "minio-certs",
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					SecretName: MinioSecretHTTPS,
					Items: []core.KeyToPath{
						{
							Key:  MINIO_PUBLIC_CRT_NAME,
							Path: MINIO_PUBLIC_CRT_NAME,
						},
						{
							Key:  MINIO_PRIVATE_KEY_NAME,
							Path: MINIO_PRIVATE_KEY_NAME,
						},
						{
							Key:  MINIO_PUBLIC_CRT_NAME,
							Path: filepath.Join("CAs", MINIO_PUBLIC_CRT_NAME),
						},
					},
				},
			},
		},
	}
	mountCA := []core.VolumeMount{
		{
			Name:      "minio-certs",
			MountPath: MINIO_CERTS_MOUNTPATH,
		},
	}

	deploy := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(MinioServerHTTPS),
			Namespace: i.namespace,
			Labels:    labels,
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},

			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// minio service will select this pod using this label.
					Labels: labels,
				},
				Spec: core.PodSpec{
					// this volumes will be mounted on minio server container
					Volumes: []core.Volume{
						{
							Name: "minio-storage",
							VolumeSource: core.VolumeSource{
								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
									ClaimName: mpvc.Name,
								},
							},
						},
					},
					// run this containers in minio server pod

					Containers: []core.Container{
						{
							Name:  MinioServerHTTPS,
							Image: "minio/minio",
							Args: []string{
								"server",
								"--address",
								":" + strconv.Itoa(PORT),
								"/storage",
							},
							Env: []core.EnvVar{
								{
									Name: MINIO_ACCESS_KEY,
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											LocalObjectReference: core.LocalObjectReference{
												Name: MinioSecretHTTPS,
											},
											Key: AWS_ACCESS_KEY_ID,
										},
									},
								},
								{
									Name: MINIO_SECRET_KEY,
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											LocalObjectReference: core.LocalObjectReference{
												Name: MinioSecretHTTPS,
											},
											Key: AWS_SECRET_ACCESS_KEY,
										},
									},
								},
							},
							Ports: []core.ContainerPort{
								{
									ContainerPort: int32(PORT),
								},
							},
							VolumeMounts: []core.VolumeMount{
								{
									Name:      "minio-storage",
									MountPath: "/storage",
								},
							},
						},
					},
				},
			},
		},
	}

	if tls {
		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, CAvol[0])
		deploy.Spec.Template.Spec.Containers[0].VolumeMounts = append(deploy.Spec.Template.Spec.Containers[0].VolumeMounts, mountCA[0])
	}
	return deploy
}

func (i *Invocation) ServiceForMinioServer() core.Service {
	labels := map[string]string{
		"app": MinioServerHTTP,
	}
	name := MinioServiceHTTP
	if MinioTLS {
		labels = map[string]string{
			"app": MinioServerHTTPS,
		}
		name = MinioServiceHTTPS
	}

	return core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: i.namespace,
			Labels:    labels,
		},
		Spec: core.ServiceSpec{
			Type: core.ServiceTypeNodePort,
			Ports: []core.ServicePort{
				{
					Port:       int32(PORT),
					TargetPort: intstr.FromInt(PORT),
					Protocol:   core.ProtocolTCP,
				},
			},
			Selector: labels,
		},
	}
}

func (i *Invocation) CreateService(obj core.Service) (*core.Service, error) {
	return i.kubeClient.CoreV1().Services(obj.Namespace).Create(context.TODO(), &obj, metav1.CreateOptions{})
}

func (i *Invocation) MinioServiceAddress() string {
	return fmt.Sprintf("%s.%s.svc:%d", MinioService, i.namespace, PORT)
}

func (f *Framework) GetMinioPortForwardingEndPoint() (*portforward.Tunnel, error) {
	tunnel, err := f.ForwardMinioPort(minioPodName)
	if err != nil {
		return nil, err
	}
	return tunnel, err
}

func (i *Invocation) MinioServerSANs() cert.AltNames {
	var myIPs []net.IP
	myIPs = append(myIPs, net.ParseIP(minikubeIP))
	myIPs = append(myIPs, net.ParseIP(localIP))
	altNames := cert.AltNames{
		DNSNames: []string{fmt.Sprintf("%s.%s.svc", MinioService, i.namespace)},
		IPs:      myIPs,
	}
	return altNames
}

func (i *Invocation) MinioServerDeploymentHTTP() *apps.Deployment {
	labels := map[string]string{
		"app": MinioServerHTTP,
	}

	deploy := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MinioServerHTTP,
			Namespace: i.namespace,
			Labels:    labels,
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},

			Strategy: apps.DeploymentStrategy{
				Type: apps.RecreateDeploymentStrategyType,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// minio service will select this pod using this label.
					Labels: labels,
				},
				Spec: core.PodSpec{
					// this volumes will be mounted on minio server container
					Volumes: []core.Volume{
						{
							Name: "minio-storage",
							VolumeSource: core.VolumeSource{
								PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
									ClaimName: mpvc.Name,
								},
							},
						},
					},
					// run this containers in minio server pod

					Containers: []core.Container{
						{
							Name:  MinioServerHTTP,
							Image: "minio/minio",
							Args: []string{
								"server",
								"--address",
								":" + strconv.Itoa(PORT),
								"/storage",
							},
							Env: []core.EnvVar{
								{
									Name: MINIO_ACCESS_KEY,
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											LocalObjectReference: core.LocalObjectReference{
												Name: MinioSecretHTTP,
											},
											Key: AWS_ACCESS_KEY_ID,
										},
									},
								},
								{
									Name: MINIO_SECRET_KEY,
									ValueFrom: &core.EnvVarSource{
										SecretKeyRef: &core.SecretKeySelector{
											LocalObjectReference: core.LocalObjectReference{
												Name: MinioSecretHTTP,
											},
											Key: AWS_SECRET_ACCESS_KEY,
										},
									},
								},
							},
							Ports: []core.ContainerPort{
								{
									ContainerPort: int32(PORT),
								},
							},
							VolumeMounts: []core.VolumeMount{
								{
									Name:      "minio-storage",
									MountPath: "/storage",
								},
							},
						},
					},
				},
			},
		},
	}

	return deploy
}

func (i *Invocation) DeleteMinioServer() {
	//wait for all postgres reources to wipeout
	err := i.DeleteSecret(mcred.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
	err = i.DeletePersistentVolumeClaim(mpvc.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
	err = i.DeleteServiceForMinioServer(msrvc.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
	err = i.DeleteDeploymentForMinioServer(mdeploy.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
}

func (f *Framework) DeleteServiceForMinioServer(meta metav1.ObjectMeta) error {
	return f.kubeClient.CoreV1().Services(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (f *Framework) DeleteDeploymentForMinioServer(meta metav1.ObjectMeta) error {
	return f.kubeClient.AppsV1().Deployments(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInBackground())
}
