package framework

import (
	"crypto/rand"
	"fmt"

	"github.com/go-xorm/xorm"
	_ "github.com/lib/pq"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *Framework) GetPostgresClient(meta metav1.ObjectMeta) (*xorm.Engine, error) {
	postgres, err := f.GetPostgres(meta)
	if err != nil {
		return nil, err
	}
	clientPodName := fmt.Sprintf("%v-0", postgres.Name)
	port, err := f.getProxyPort(postgres.Namespace, clientPodName, 5432)
	if err != nil {
		return nil, err
	}

	cnnstr := fmt.Sprintf("user=postgres host=127.0.0.1 port=%v dbname=postgres sslmode=disable", port)
	return xorm.NewEngine("postgres", cnnstr)
}

func (f *Framework) CreateSchema(db *xorm.Engine) error {
	sql := `
DROP SCHEMA IF EXISTS "data" CASCADE;
CREATE SCHEMA "data" AUTHORIZATION "postgres";
`
	_, err := db.Exec(sql)
	if err != nil {
		return err
	}
	return nil
}

var randChars = []rune("abcdefghijklmnopqrstuvwxyzabcdef")

// Use this for generating random pat of a ID. Do not use this for generating short passwords or secrets.
func characters(len int) string {
	bytes := make([]byte, len)
	rand.Read(bytes)
	r := make([]rune, len)
	for i, b := range bytes {
		r[i] = randChars[b>>3]
	}
	return string(r)
}

func (f *Framework) CreateTable(db *xorm.Engine, count int) error {

	for i := 0; i < count; i++ {
		table := fmt.Sprintf("SET search_path TO \"data\"; CREATE TABLE %v ( id bigserial )", characters(5))
		_, err := db.Exec(table)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) CountTable(db *xorm.Engine) (int, error) {
	res, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema='data'")
	if err != nil {
		return 0, err
	}

	return len(res), nil
}

func (f *Framework) CheckPostgres(db *xorm.Engine) error {
	err := db.Ping()
	if err != nil {
		return err
	}
	return nil
}

type PgStatArchiver struct {
	ArchivedCount int
}

func (f *Framework) CountArchive(db *xorm.Engine) (int, error) {
	var archiver PgStatArchiver
	if _, err := db.Limit(1).Cols("archived_count").Get(&archiver); err != nil {
		return 0, err
	}

	return archiver.ArchivedCount, nil
}
