package framework

import (
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	"gomodules.xyz/stow"
	storage "kmodules.xyz/objectstore-api/osm"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/postgres/pkg/controller"
)

func (f *Framework) EventuallyWalDataFound(postgres *api.Postgres) GomegaAsyncAssertion {
	if f.IsMinio() { // if it is minio
		return Eventually(
			func() bool {
				found, err := f.checkMinioWalData(postgres)
				Expect(err).NotTo(HaveOccurred())
				return found
			},
			time.Minute*5,
			time.Second*5,
		)
	} else {
		return Eventually(
			func() bool {
				found, err := f.checkWalData(postgres)
				Expect(err).NotTo(HaveOccurred())
				return found
			},
			time.Minute*5,
			time.Second*5,
		)
	}

}

func (f *Framework) checkWalData(postgres *api.Postgres) (bool, error) {
	cfg, err := storage.NewOSMContext(f.kubeClient, *postgres.Spec.Archiver.Storage, postgres.Namespace)
	if err != nil {
		return false, err
	}

	loc, err := stow.Dial(cfg.Provider, cfg.Config)
	if err != nil {
		return false, err
	}
	containerID, err := postgres.Spec.Archiver.Storage.Container()
	if err != nil {
		return false, err
	}
	container, err := loc.Container(containerID)
	if err != nil {
		return false, err
	}

	prefix := controller.WalDataDir(postgres)
	cursor := stow.CursorStart
	totalItem := 0
	for {
		items, next, err := container.Items(prefix, cursor, 50)
		if err != nil {
			return false, err
		}

		totalItem = totalItem + len(items)

		cursor = next
		if stow.IsCursorEnd(cursor) {
			break
		}
	}

	return totalItem != 0, nil
}

func (f *Framework) checkMinioWalData(postgres *api.Postgres) (bool, error) {
	tunnel, err := f.GetMinioPortForwardingEndPoint()
	//if tunnel.Local != 0{
	//	endPoint := fmt.Sprintf("https://%s:%d", localIP, tunnel.Local)
	//}
	endPoint := ""
	if f.IsTLS() {
		endPoint = fmt.Sprintf("https://%s:%d", localIP, tunnel.Local)
	} else {
		endPoint = fmt.Sprintf("http://%s:%d", localIP, tunnel.Local)
	}

	if err != nil {
		return false, err
	}
	if postgres.Spec.Archiver.Storage != nil {
		if postgres.Spec.Archiver.Storage.S3 != nil {
			postgres.Spec.Archiver.Storage.S3.Endpoint = endPoint
		}
	}
	walBool, err := f.checkWalData(postgres)
	defer tunnel.Close()
	if err != nil {
		return false, err
	}
	return walBool, nil
}
