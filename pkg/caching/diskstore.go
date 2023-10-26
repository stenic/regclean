package caching

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	lib_store "github.com/eko/gocache/lib/v4/store"
	"github.com/gofrs/flock"
	"github.com/sirupsen/logrus"
)

const (
	DiskCacheType = "disk-cache"
)

type DiskStore[T any] struct {
	baseDir string
}

func (store DiskStore[T]) keyPath(key string) string {
	return path.Join(store.baseDir, key) + ".cache"
}

func (store DiskStore[T]) Get(ctx context.Context, key any) (any, error) {
	filename := store.keyPath(key.(string))
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		return nil, lib_store.NotFound{}
	}
	lock := flock.New(filename)
	defer func() {
		if err := lock.Unlock(); err != nil {
			logrus.Warnf("Unable to unlock file %s: %v", filename, err)
		}
	}()

	var cacheValue []byte
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	ok, err := lock.TryRLockContext(ctx, 250*time.Millisecond) // try to lock every 1/4 second
	if !ok {
		// unable to lock the cache, something is wrong, refuse to use it.
		return cacheValue, fmt.Errorf("unable to read lock file %s: %v", filename, err)
	}

	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return cacheValue, fmt.Errorf("failed to read cache file: %w", err)
	}
	buffer := bytes.Buffer{}
	buffer.Write(fileBytes)
	d := gob.NewDecoder(&buffer)
	data := new(T)
	if err := d.Decode(&data); err != nil {
		return cacheValue, fmt.Errorf("failed to decode cache file: %w", err)
	}

	return *data, nil

}
func (store DiskStore[T]) GetWithTTL(ctx context.Context, key any) (any, time.Duration, error) {
	panic("implement me")
}
func (store DiskStore[T]) Set(ctx context.Context, key any, value any, options ...lib_store.Option) error {
	filename := store.keyPath(key.(string))
	lock := flock.New(filename)
	defer func() {
		if err := lock.Unlock(); err != nil {
			logrus.Warnf("Unable to unlock file %s: %v\n", filename, err)
		}
	}()

	// wait up to a second for the file to lock
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	ok, err := lock.TryRLockContext(ctx, 250*time.Millisecond) // try to lock every 1/4 second
	if !ok {
		// unable to lock the cache, something is wrong, refuse to use it.
		return fmt.Errorf("unable to read lock file %s: %v", filename, err)
	}
	var buffer bytes.Buffer
	err = gob.NewEncoder(&buffer).Encode(value)
	if err == nil {
		// write privately owned by the user
		err = os.WriteFile(filename, buffer.Bytes(), 0600)
	}
	return err
}
func (store DiskStore[T]) Delete(ctx context.Context, key any) error {
	panic("implement me")
}
func (store DiskStore[T]) Invalidate(ctx context.Context, options ...lib_store.InvalidateOption) error {
	panic("implement me")
}
func (store DiskStore[T]) Clear(ctx context.Context) error {
	panic("implement me")
}
func (store DiskStore[T]) GetType() string {
	return DiskCacheType
}
