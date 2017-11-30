package framework

import (
	"crypto/rand"
	"fmt"

	"github.com/go-pg/pg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (f *Framework) GetPostgresClient(meta metav1.ObjectMeta) (*pg.DB, error) {
	postgres, err := f.GetPostgres(meta)
	if err != nil {
		return nil, err
	}
	clientPodName := fmt.Sprintf("%v-0", postgres.Name)
	url, err := f.getProxyURL(postgres.Namespace, clientPodName, 5432)
	if err != nil {
		return nil, err
	}

	return pg.Connect(&pg.Options{
		Addr:     url,
		Database: "postgres",
		User:     "postgres",
		Password: "postgres",
	}), nil
}

func (f *Framework) CreateSchema(db *pg.DB) error {
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

func (f *Framework) CreateTable(db *pg.DB, count int) error {

	for i := 0; i < count; i++ {
		table := fmt.Sprintf("SET search_path TO \"data\"; CREATE TABLE %v ( id bigserial )", characters(5))
		fmt.Println(table)
		_, err := db.Exec(table)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) CountTable(db *pg.DB) (int, error) {
	res, err := db.Exec("SELECT table_name FROM information_schema.tables WHERE table_schema='data'")
	if err != nil {
		return 0, err
	}

	return res.RowsReturned(), nil
}

func (f *Framework) CheckPostgres(db *pg.DB) error {
	_, err := db.Exec("select * from pg_stat_activity")
	if err != nil {
		return err
	}
	return nil
}
