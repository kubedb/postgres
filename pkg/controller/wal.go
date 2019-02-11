package controller

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/graymeta/stow"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/objectstore-api/osm"
)

func WalDataDir(postgres *api.Postgres) string {
	spec := postgres.Spec.Archiver.Storage
	if spec.S3 != nil {
		return filepath.Join(spec.S3.Prefix, api.DatabaseNamePrefix, postgres.Namespace, postgres.Name, "archive")
	} else if spec.GCS != nil {
		return filepath.Join(spec.GCS.Prefix, api.DatabaseNamePrefix, postgres.Namespace, postgres.Name, "archive")
	} else if spec.Azure != nil {
		return filepath.Join(spec.Azure.Prefix, api.DatabaseNamePrefix, postgres.Namespace, postgres.Name, "archive")
	} else if spec.Swift != nil {
		return filepath.Join(spec.Swift.Prefix, api.DatabaseNamePrefix, postgres.Namespace, postgres.Name, "archive")
	}
	return ""
}

func (c *Controller) wipeOutWalData(meta metav1.ObjectMeta, spec *api.PostgresSpec) error {
	if spec == nil {
		return fmt.Errorf("wipeout wal data failed. Reason: invalid postgres spec")
	}

	postgres := &api.Postgres{
		ObjectMeta: meta,
		Spec:       *spec,
	}

	if postgres.Spec.Archiver == nil {
		log.Println("====================================> Achiever is nil")
		// no archiver was configured. nothing to remove.
		return nil
	}

	cfg, err := osm.NewOSMContext(c.Client, *postgres.Spec.Archiver.Storage, postgres.Namespace)
	if err != nil {
		log.Println("====================================> Cant get cfg")
		return err
	}

	loc, err := stow.Dial(cfg.Provider, cfg.Config)
	if err != nil {
		log.Println("====================================> Cant dial stow")
		return err
	}
	bucket, err := postgres.Spec.Archiver.Storage.Container()
	if err != nil {
		log.Println("====================================> Cant get Bucket")
		return err
	}
	container, err := loc.Container(bucket)
	if err != nil {
		log.Println("====================================> Cant get Container")
		return err
	}

	prefix := WalDataDir(postgres)
	cursor := stow.CursorStart
	for {
		log.Println("====================================> In the loop with cursors")
		items, next, err := container.Items(prefix, cursor, 50)
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := container.RemoveItem(item.ID()); err != nil {
				return err
			}
		}
		cursor = next
		if stow.IsCursorEnd(cursor) {
			break
		}
	}

	return nil
}
