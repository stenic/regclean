package caching

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/eko/gocache/lib/v4/store"
	lib_store "github.com/eko/gocache/lib/v4/store"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"

	"github.com/jmoiron/sqlx"
)

const (
	SQLLiteStoreType = "sqllite-cache"
)

type SQLLiteStore[T any] struct {
	db *sqlx.DB
}

type dbRec struct {
	Key  string `db:"key"`
	Data []byte `db:"data"`
}

func NewSQLLiteStore[T any]() store.StoreInterface {
	cacheDir := "./.cache"
	os.Mkdir(cacheDir, 0775)

	needsInit := false
	dbFile := cacheDir + "/cache.db"
	if _, err := os.Stat(dbFile); errors.Is(err, os.ErrNotExist) {
		needsInit = true
	}

	logrus.Debug("Opening cache database")

	db, err := sqlx.Connect("sqlite3", dbFile)
	if err != nil {
		logrus.Fatal(err)
	}

	if needsInit {
		logrus.Debug("Creating schema")
		db.MustExec(`create table cache (
			key text not null primary key,
			data blob
		)`)
	}

	return &SQLLiteStore[T]{
		db: db,
	}
}

func (store SQLLiteStore[T]) Get(ctx context.Context, key any) (any, error) {
	data := dbRec{}
	if err := store.db.Get(&data, "SELECT * FROM cache WHERE key = $1", key.(string)); err != nil {
		return nil, lib_store.NotFound{}
	}

	var cacheValue []byte
	buffer := bytes.Buffer{}
	buffer.Write(data.Data)
	d := gob.NewDecoder(&buffer)
	result := new(T)
	if err := d.Decode(&result); err != nil {
		return cacheValue, fmt.Errorf("failed to decode cache file: %w", err)
	}

	return *result, nil
}

func (store SQLLiteStore[T]) GetWithTTL(ctx context.Context, key any) (any, time.Duration, error) {
	panic("implement me")
}
func (store SQLLiteStore[T]) Set(ctx context.Context, key any, value any, options ...lib_store.Option) error {
	var buffer bytes.Buffer
	if err := gob.NewEncoder(&buffer).Encode(value); err != nil {
		return err
	}
	_, err := store.db.NamedExec(
		`INSERT INTO cache (key, data) 
		VALUES (:key, :data) 
		ON CONFLICT(key) DO UPDATE SET data=excluded.data`,
		&dbRec{
			Key:  key.(string),
			Data: buffer.Bytes(),
		})

	return err
}
func (store SQLLiteStore[T]) Delete(ctx context.Context, key any) error {
	panic("implement me")
}
func (store SQLLiteStore[T]) Invalidate(ctx context.Context, options ...lib_store.InvalidateOption) error {
	panic("implement me")
}
func (store SQLLiteStore[T]) Clear(ctx context.Context) error {
	panic("implement me")
}
func (store SQLLiteStore[T]) GetType() string {
	return SQLLiteStoreType
}
