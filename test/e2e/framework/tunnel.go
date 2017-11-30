package framework

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func (f *Framework) getProxyPort(namespace, podName string, port int) (int, error) {
	tunnel := newTunnel(f.kubeClient, f.restConfig, namespace, podName, port)
	if err := tunnel.forwardPort(); err != nil {
		return 0, err
	}

	return tunnel.Local, nil
}

type tunnel struct {
	Local      int
	Remote     int
	Namespace  string
	PodName    string
	Out        io.Writer
	stopChan   chan struct{}
	readyChan  chan struct{}
	config     *rest.Config
	kubeClient kubernetes.Interface
}

func newTunnel(client kubernetes.Interface, config *rest.Config, namespace, podName string, remote int) *tunnel {
	return &tunnel{
		config:     config,
		kubeClient: client,
		Namespace:  namespace,
		PodName:    podName,
		Remote:     remote,
		stopChan:   make(chan struct{}, 1),
		readyChan:  make(chan struct{}, 1),
		Out:        ioutil.Discard,
	}
}

func (t *tunnel) forwardPort() error {
	u := t.kubeClient.Core().RESTClient().Post().
		Resource("pods").
		Namespace(t.Namespace).
		Name(t.PodName).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(t.config)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", u)

	local, err := getAvailablePort()
	if err != nil {
		return fmt.Errorf("could not find an available port: %s", err)
	}
	t.Local = local

	ports := []string{fmt.Sprintf("%d:%d", t.Local, t.Remote)}

	pf, err := portforward.New(dialer, ports, t.stopChan, t.readyChan, t.Out, t.Out)
	if err != nil {
		return err
	}

	errChan := make(chan error)
	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case err = <-errChan:
		return fmt.Errorf("forwarding ports: %v", err)
	case <-pf.Ready:
		return nil
	}
}

func getAvailablePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()

	_, p, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	return port, err
}
