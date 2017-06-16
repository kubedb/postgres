package dump

import (
	"encoding/json"
	"fmt"

	"github.com/go-ini/ini"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	"github.com/k8sdb/postgres/pkg/audit/dump/client"
	"github.com/k8sdb/postgres/pkg/audit/dump/lib"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

func PostgresAudit(
	kubeClient clientset.Interface,
	dbClient tcs.ExtensionInterface,
	namespace string,
	kubedbName string,
	dbname string,
) ([]byte, error) {
	postgres, err := dbClient.Postgreses(namespace).Get(kubedbName)
	if err != nil {
		return nil, err
	}

	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(postgres.Spec.DatabaseSecret.SecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	cfg, err := ini.Load(secret.Data[".admin"])
	if err != nil {
		return nil, err
	}
	section, err := cfg.GetSection("")
	if err != nil {
		return nil, err
	}
	username := "postgres"
	if k, err := section.GetKey("POSTGRES_USER"); err == nil {
		username = k.Value()
	}
	var password string
	if k, err := section.GetKey("POSTGRES_PASSWORD"); err == nil {
		password = k.Value()
	}

	host := fmt.Sprintf("%v.%v", kubedbName, namespace)
	port := "5432"

	databases := make([]string, 0)

	if dbname == "" {
		engine, err := client.NewEngine(username, password, host, port, "postgres")
		if err != nil {
			return nil, err
		}
		databases = lib.GetAllDatabase(engine)
	} else {
		databases = append(databases, dbname)
	}

	dbs := make(map[string]*lib.DBInfo)
	for _, db := range databases {
		engine, err := client.NewEngine(username, password, host, port, db)
		if err != nil {
			return nil, err
		}
		dbInfo, err := lib.DumpDBInfo(engine)
		if err != nil {
			return nil, err
		}

		dbs[db] = dbInfo
	}

	return json.MarshalIndent(dbs, "", "  ")
}
