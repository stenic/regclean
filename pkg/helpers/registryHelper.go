package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regclean/pkg/caching"
	"strings"
	"time"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/sirupsen/logrus"

	"github.com/heroku/docker-registry-client/registry"
)

type regHelper struct {
	hub          *registry.Registry
	RegPrefix    string
	dryRun       bool
	cache        map[string]imageMeta
	cacheManager *cache.Cache[imageMeta]
}

func NewRegHelper(URL, username, password string, dryRun bool) *regHelper {
	URL = strings.TrimSuffix(URL, "/")
	hub := &registry.Registry{
		URL: URL,
		Client: &http.Client{
			Transport: registry.WrapTransport(
				http.DefaultTransport,
				URL,
				username,
				password,
			),
		},
		Logf: func(format string, args ...interface{}) {
			logrus.Tracef(format, args...)
		},
	}

	if err := hub.Ping(); err != nil {
		logrus.Fatal(err)
	}

	u, err := url.ParseRequestURI(URL)
	if err != nil {
		logrus.Fatal(err)
	}
	regPrefix := u.Host

	return &regHelper{
		hub:          hub,
		RegPrefix:    regPrefix,
		cache:        map[string]imageMeta{},
		cacheManager: caching.NewCache[imageMeta](),
		dryRun:       dryRun,
	}
}

func (h regHelper) GetImages() []string {
	repos, err := h.hub.Repositories()
	if err != nil {
		logrus.Fatal(err)
	}

	images := []string{}
	for _, repo := range repos {
		tags, err := h.hub.Tags(repo)
		if err != nil {
			logrus.Fatal(err)
		}
		for _, tag := range tags {
			images = append(images, fmt.Sprintf("%s/%s:%s", h.RegPrefix, repo, tag))
		}
	}

	return images
}

func (h regHelper) splitImageTag(image string) (string, string) {
	i := strings.Split(strings.TrimPrefix(image, h.RegPrefix+"/"), ":")
	return i[0], i[1]
}

func (h regHelper) DeleteImage(image string) error {
	img, tag := h.splitImageTag(image)
	digest, err := h.hub.ManifestDigest(img, tag)
	if err != nil {
		return fmt.Errorf("failed to fetch digest: %w", err)
	}

	if h.dryRun {
		logrus.Infof("Dry run, skipping delete of %s:%s (%s) on registry", img, tag, digest.String())
		return nil
	}

	logrus.Warnf("Deleting %s:%s (%s) on registry", img, tag, digest.String())
	if err = h.hub.DeleteManifest(img, digest); err != nil {
		return fmt.Errorf("failed to delete manifest: %w", err)
	}
	return nil
}

type blobResponse struct {
	Created time.Time `json:"created"`
}

type imageMeta struct {
	Created   time.Time
	TotalSize uint64
}

func (h regHelper) imageMeta(img, tag string) (*imageMeta, error) {
	key := img + ":" + tag
	if meta, err := h.cacheManager.Get(context.TODO(), key); err == nil {
		return &meta, nil
	}

	manifest, err := h.hub.ManifestV2(img, tag)
	if err != nil {
		logrus.Warn(err)
		return nil, err
	}

	blob, err := h.hub.DownloadBlob(img, manifest.Config.Digest)
	if err != nil {
		logrus.Warn(err)
		return nil, err
	}
	bytes, err := io.ReadAll(blob)
	if err != nil {
		logrus.Warn(err)
		return nil, err
	}
	var blobResp blobResponse
	err = json.Unmarshal(bytes, &blobResp)
	if err != nil {
		logrus.Warn(err)
		return nil, err
	}

	total := uint64(manifest.Config.Size)
	for _, layer := range manifest.Layers {
		total += uint64(layer.Size)
	}

	meta := imageMeta{
		Created:   blobResp.Created,
		TotalSize: total,
	}
	// h.cache[key] = meta
	h.cacheManager.Set(context.Background(), key, meta)
	return &meta, nil
}

func (h regHelper) GetImageSize(image string) (uint64, error) {
	img, tag := h.splitImageTag(image)
	meta, err := h.imageMeta(img, tag)
	if err != nil {
		return 0, err
	}
	return meta.TotalSize, err
}

func (h regHelper) GetImageDate(image string) (*time.Time, error) {
	img, tag := h.splitImageTag(image)
	meta, err := h.imageMeta(img, tag)
	if err != nil {
		return nil, err
	}
	return &meta.Created, nil
}
