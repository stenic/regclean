package helpers

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/utils/strings/slices"
)

type filterHelper struct {
	rh                 *regHelper
	ExcludeNameFilters []string
	IncludeNameFilters []string
	MinAge             int
}

func NewFilterHelper(rh regHelper) *filterHelper {
	return &filterHelper{
		rh: &rh,
	}
}

func (h filterHelper) FilterImages(images []string) ([]string, int) {
	filtered := []string{}

	stringContains := func(img string) func(s string) bool {
		return func(s string) bool {
			return !strings.Contains(img, s)
		}
	}

	stats := map[string]int{
		"exclude_name": 0,
		"include_name": 0,
		"min_age":      0,
	}
	logrus.WithFields(logrus.Fields{
		"exclude_name": h.ExcludeNameFilters,
		"include_name": h.IncludeNameFilters,
		"min_age":      h.MinAge,
	}).Debugf("Filters")
	for _, image := range images {
		img, _ := h.rh.splitImageTag(image)

		// Filter by include-name-filters
		if len(h.IncludeNameFilters) > 0 && len(slices.Filter(nil, h.IncludeNameFilters, stringContains(img))) == len(h.IncludeNameFilters) {
			logrus.Tracef("Image %s filtered by include filter, skipping", img)
			stats["include_name"]++
			continue
		}

		// Filter by exclude-name-filters
		if len(h.ExcludeNameFilters) > 0 && len(slices.Filter(nil, h.ExcludeNameFilters, stringContains(img))) != len(h.ExcludeNameFilters) {
			logrus.Tracef("Image %s filtered by exclude filter, skipping", img)
			stats["exclude_name"]++
			continue
		}

		// Filter by minimum age
		if created, err := h.rh.GetImageDate(image); err == nil && created.After(time.Now().AddDate(0, 0, -h.MinAge)) {
			logrus.Tracef("Image %s is younger than %d days, skipping", image, h.MinAge)
			stats["min_age"]++
			continue
		}
		filtered = append(filtered, image)
	}

	logrus.WithFields(logrus.Fields{
		"exclude_name": stats["exclude_name"],
		"include_name": stats["include_name"],
		"min_age":      stats["min_age"],
	}).Debug("Filter stats")

	return filtered, len(images) - len(filtered)
}
