package caching

import (
	"os"

	"github.com/eko/gocache/lib/v4/cache"
)

func NewCache[T any]() *cache.Cache[T] {
	cacheDir := "./.cache"
	os.Mkdir(cacheDir, 0775)

	return cache.New[T](DiskStore[T]{
		baseDir: cacheDir,
	})
}
