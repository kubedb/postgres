package framework

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/appscode/go/crypto/rand"
	"gomodules.xyz/cert"
	"gomodules.xyz/stow"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	apps_util "kmodules.xyz/client-go/apps/v1"
	v12 "kmodules.xyz/client-go/core/v1"
	"kmodules.xyz/client-go/tools/portforward"
	v1 "kmodules.xyz/objectstore-api/api/v1"
	"kmodules.xyz/objectstore-api/osm"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
)

const (
	MINIO_PUBLIC_CRT_NAME  = "public.crt"
	MINIO_PRIVATE_KEY_NAME = "private.key"

	MINIO_ACCESS_KEY = "MINIO_ACCESS_KEY"
	MINIO_SECRET_KEY = "MINIO_SECRET_KEY"

	AWS_ACCESS_KEY_ID     = "AWS_ACCESS_KEY_ID"
	AWS_SECRET_ACCESS_KEY = "AWS_SECRET_ACCESS_KEY"

	MINIO_CERTS_MOUNTPATH = "/root/.minio/certs"
	StandardStorageClass  = "standard"
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
	mcred   *core.Secret
	mpvc    *core.PersistentVolumeClaim
	mdeploy *apps.Deployment
	msrvc   core.Service
	//postgres *api.Postgres
	postgres      *api.Postgres
	TLS           bool
	clientPodName = ""
	MinioWAL      = false
	MinioService  = ""
)

func (fi *Invocation) CreateMinioServer(tls bool, ips []net.IP) (string, error) {
	TLS = tls
	MinioWAL = true
	//creating service for minio server
	var err error
	if TLS {
		err = fi.CreateHTTPSMinioServer()
	} else {
		err = fi.CreateHTTPMinioServer()
	}
	if err != nil {
		return "", err
	}

	var endPoint string
	if tls {
		endPoint = "https://" + fi.MinioServiceAddress()
	} else {
		endPoint = "http://" + fi.MinioServiceAddress()
	}
	return endPoint, nil
}

func (fi *Invocation) CreateHTTPMinioServer() error {
	msrvc = fi.ServiceForMinioServer()
	MinioService = MinioServiceHTTP
	_, err := fi.CreateServiceForMinioServer(msrvc)
	if err != nil {
		return err
	}
	//removing previous depolyment if exists
	if !TLS {
		err = v12.WaitUntillPodTerminatedByLabel(fi.kubeClient, fi.namespace, labels.Set{"app": MinioServerHTTPS}.String())
	}
	//creating secret for minio server
	mcred = fi.SecretForS3Backend()
	mcred.Name = MinioSecretHTTP
	secret, err := fi.CreateMinioSecret(mcred)
	if err != nil {
		return err
	}

	//creating pvc for minio server
	mpvc = fi.GetPersistentVolumeClaim()
	mpvc.Name = MinioPVC
	mpvc.Labels = map[string]string{"app": "minio-storage-claim"}

	err = fi.CreatePersistentVolumeClaim(mpvc)
	if err != nil {
		return nil
	}
	//creating deployment for minio server
	myDeploy := fi.MinioServerDeploymentHTTP()
	// if tls not enabled then don't mount secret for cacerts
	//mdeploy.Spec.Template.Spec.Containers = fi.RemoveSecretVolumeMount(mdeploy.Spec.Template.Spec.Containers)
	deploy, err := fi.CreateDeploymentForMinioServer(myDeploy)
	if err != nil {
		return err
	}
	err = apps_util.WaitUntilDeploymentReady(fi.kubeClient, deploy.ObjectMeta)
	if err != nil {
		return err
	}

	err = fi.CreateBucket(deploy, secret, false)
	if err != nil {
		return err
	}
	return nil
}

func (fi *Invocation) CreateHTTPSMinioServer() error {
	msrvc = fi.ServiceForMinioServer()
	MinioService = MinioServiceHTTPS
	_, err := fi.CreateServiceForMinioServer(msrvc)
	if err != nil {
		return err
	}

	//creating secret with CA for minio server
	mcred = fi.SecretForMinioServer()

	mcred.Name = MinioSecretHTTPS
	secret, err := fi.CreateMinioSecret(mcred)
	if err != nil {
		return err
	}

	//creating pvc for minio server
	mpvc = fi.GetPersistentVolumeClaim()
	mpvc.Name = MinioPVC
	mpvc.Labels = map[string]string{"app": "minio-storage-claim"}

	err = fi.CreatePersistentVolumeClaim(mpvc)
	if err != nil {
		return nil
	}

	//creating deployment for minio server
	mdeploy = fi.MinioServerDeploymentHTTPS(true)
	deploy, err := fi.CreateDeploymentForMinioServer(mdeploy)
	if err != nil {
		return err
	}

	err = apps_util.WaitUntilDeploymentReady(fi.kubeClient, deploy.ObjectMeta)
	if err != nil {
		return err
	}

	err = fi.CreateBucket(deploy, secret, true)
	if err != nil {
		return err
	}
	return nil
}

func (f *Framework) IsTLS() bool {
	return TLS
}
func (f *Framework) IsMinio() bool {
	return MinioWAL
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

func (fi *Invocation) CreateBucket(deployment *apps.Deployment, secret *core.Secret, tls bool) error {
	endPoint := ""
	podlist, err := fi.kubeClient.CoreV1().Pods(deployment.ObjectMeta.Namespace).List(metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(deployment.Spec.Selector)})
	if err != nil {
		return err
	}
	if len(podlist.Items) > 0 {
		for _, pod := range podlist.Items {
			clientPodName = pod.Name
			break
		}
	}

	if tls {
		sec := fi.SecretForMinioBackend()
		sec.Name = "mock-s3-secret"
		secret, err = fi.CreateMinioSecret(sec)
		if err != nil {
			return err
		}
	}

	tunnel, err := fi.ForwardMinioPort(clientPodName)
	if err != nil {
		return err
	}

	err = wait.PollImmediate(2*time.Second, 10*time.Minute, func() (bool, error) {
		if tls {
			endPoint = fmt.Sprintf("https://%s:%d", localIP, tunnel.Local)
		} else {
			endPoint = fmt.Sprintf("http://%s:%d", localIP, tunnel.Local)
		}
		err = fi.CreateMinioBucket(os.Getenv(S3_BUCKET_NAME), secret, endPoint)
		if err != nil {
			return false, nil //dont return error
		}
		defer tunnel.Close()
		return true, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (fi *Invocation) CreateMinioBucket(bucketName string, secret *core.Secret, endPoint string) error {
	postgres = fi.Postgres()
	postgres.Spec.Archiver = &api.PostgresArchiverSpec{
		Storage: &v1.Backend{
			StorageSecretName: secret.Name,
			S3: &v1.S3Spec{
				Bucket:   os.Getenv(S3_BUCKET_NAME),
				Endpoint: endPoint,
			},
		},
	}
	cfg, err := osm.NewOSMContext(fi.kubeClient, *postgres.Spec.Archiver.Storage, fi.Namespace())
	if err != nil {
		return err
	}

	loc, err := stow.Dial(cfg.Provider, cfg.Config)
	if err != nil {
		return err
	}

	containerID, err := postgres.Spec.Archiver.Storage.Container()
	if err != nil {
		return err
	}
	_, err = loc.CreateContainer(containerID)
	if err != nil {
		return err
	}

	return nil
}

func (fi *Invocation) CreateDeploymentForMinioServer(obj *apps.Deployment) (*apps.Deployment, error) {
	newDeploy, err := fi.kubeClient.AppsV1().Deployments(obj.Namespace).Create(obj)
	return newDeploy, err
}

func (fi *Invocation) MinioServerDeploymentHTTPS(tls bool) *apps.Deployment {
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
			Namespace: fi.namespace,
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
									ClaimName: MinioPVC,
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

func (fi *Invocation) ServiceForMinioServer() core.Service {
	var labels map[string]string
	var name string
	if TLS {
		labels = map[string]string{
			"app": MinioServerHTTPS,
		}
		name = MinioServiceHTTPS
	} else {
		labels = map[string]string{
			"app": MinioServerHTTP,
		}
		name = MinioServiceHTTP
	}

	return core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fi.namespace,
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

func (fi *Invocation) CreateServiceForMinioServer(obj core.Service) (*core.Service, error) {
	//TODO :svc, err := fi.kubeClient.CoreV1().Services(fi.namespace).Get(obj.Name, metav1.GetOptions{})
	//service, err := fi.kubeClient.CoreV1().Services(fi.namespace).Get(obj.Name, metav1.GetOptions{})
	//if err == nil {
	//	return service, nil
	//}
	newService, err := fi.kubeClient.CoreV1().Services(obj.Namespace).Create(&obj)
	return newService, err
}

func (f *Framework) CreateMinioSecret(obj *core.Secret) (*core.Secret, error) {
	secret, err := f.kubeClient.CoreV1().Secrets(obj.Namespace).Get(obj.Name, metav1.GetOptions{})
	if err == nil {
		return secret, nil
	}
	newSecret, err := f.kubeClient.CoreV1().Secrets(obj.Namespace).Create(obj)
	return newSecret, err
}

func (fi *Invocation) MinioServiceAddress() string {
	return fmt.Sprintf("%s.%s.svc:%d", MinioService, fi.namespace, PORT)
}

func (f *Framework) GetMinioPortForwardingEndPoint() (*portforward.Tunnel, error) {
	tunnel, err := f.ForwardMinioPort(clientPodName)
	if err != nil {
		return nil, err
	}
	return tunnel, err
}

func (fi *Invocation) MinioServerSANs() cert.AltNames {
	var myIPs []net.IP
	myIPs = append(myIPs, net.ParseIP(minikubeIP))
	myIPs = append(myIPs, net.ParseIP(localIP))
	altNames := cert.AltNames{
		DNSNames: []string{fmt.Sprintf("%s.%s.svc", MinioService, fi.namespace)},
		IPs:      myIPs,
	}
	return altNames
}

func (fi *Invocation) MinioServerDeploymentHTTP() *apps.Deployment {
	labels := map[string]string{
		"app": MinioServerHTTP,
	}

	deploy := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MinioServerHTTP,
			Namespace: fi.namespace,
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
									ClaimName: MinioPVC,
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

func (fi *Invocation) DeleteMinioServer() (err error) {
	//wait for all postgres reources to wipeout
	err = fi.DeleteSecretForMinioServer(mcred.ObjectMeta)
	err = fi.DeletePVCForMinioServer(mpvc.ObjectMeta)
	err = fi.DeleteDeploymentForMinioServer(mdeploy.ObjectMeta)
	err = fi.DeleteServiceForMinioServer(msrvc.ObjectMeta)
	return err
}

func (f *Framework) DeleteSecretForMinioServer(meta metav1.ObjectMeta) error {
	return f.kubeClient.CoreV1().Secrets(meta.Namespace).Delete(meta.Name, deleteInForeground())
}

func (f *Framework) DeletePVCForMinioServer(meta metav1.ObjectMeta) error {
	return f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).Delete(meta.Name, deleteInForeground())
}

func (f *Framework) DeleteDeploymentForMinioServer(meta metav1.ObjectMeta) error {
	return f.kubeClient.AppsV1().Deployments(meta.Namespace).Delete(meta.Name, deleteInBackground())
}

func (f *Framework) DeleteServiceForMinioServer(meta metav1.ObjectMeta) error {
	return f.kubeClient.CoreV1().Services(meta.Namespace).Delete(meta.Name, deleteInForeground())
}
