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
	"strings"

	"github.com/appscode/go/crypto/rand"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (i *Invocation) ConfigMapForInitialization() *core.ConfigMap {
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "-sql"),
			Namespace: i.namespace,
		},
		Data: map[string]string{
			"data.sql": `DROP SCHEMA IF EXISTS "data" CASCADE;
CREATE SCHEMA "data" AUTHORIZATION "postgres";
SET search_path TO "data";
START TRANSACTION;
SET standard_conforming_strings=off;
SET escape_string_warning=off;
SET CONSTRAINTS ALL DEFERRED;
CREATE TABLE dashboard (
    id bigserial,
    version integer NOT NULL,
    slug character varying(255) NOT NULL,
    title character varying(255) NOT NULL,
    data text NOT NULL,
    org_id bigint NOT NULL,
    created timestamp without time zone NOT NULL,
    updated timestamp without time zone NOT NULL,
    updated_by integer,
    created_by integer,
    PRIMARY KEY ("id"),
    UNIQUE (org_id, slug)
);
-- Owner-Alter-Table --
ALTER TABLE "dashboard" OWNER TO "postgres";
-- Post-data save --
COMMIT;
START TRANSACTION;
-- Sequences --
-- Full Text keys --
COMMIT;`,
		},
	}
}

func (i *Invocation) GetCustomConfig(configs []string) *core.ConfigMap {
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app + "pgconfig"),
			Namespace: i.namespace,
		},
		Data: map[string]string{
			"user.conf": strings.Join(configs, "\n"),
		},
	}
}

func (i *Invocation) CreateConfigMap(obj *core.ConfigMap) error {
	_, err := i.kubeClient.CoreV1().ConfigMaps(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) DeleteConfigMap(meta metav1.ObjectMeta) error {
	err := f.kubeClient.CoreV1().ConfigMaps(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
	if !kerr.IsNotFound(err) {
		return err
	}
	return nil
}
