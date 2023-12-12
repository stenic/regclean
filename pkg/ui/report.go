package ui

import (
	"fmt"
	"strings"

	"github.com/rodaine/table"
)

type RepoCount struct {
	Registry   string
	Repository string
	Count      int
}

func PrintCountByRepository(images []string) {
	counts := map[string]RepoCount{}

	tbl := table.New("Registry", "Repository", "Count")
	for _, image := range images {
		reg, repo, _ := splitImageTag(image)
		if entry, ok := counts[reg+"|"+repo]; ok {
			entry.Count = entry.Count + 1
			counts[reg+"|"+repo] = entry

		} else {
			counts[reg+"|"+repo] = RepoCount{
				Registry:   reg,
				Repository: repo,
				Count:      1,
			}
		}
	}

	table.DefaultHeaderFormatter = func(format string, vals ...interface{}) string {
		return strings.ToUpper(fmt.Sprintf(format, vals...))
	}

	for _, c := range counts {
		tbl.AddRow(c.Registry, c.Repository, c.Count)
	}

	tbl.Print()
}

func splitImageTag(image string) (string, string, string) {
	r := strings.SplitN(image, "/", 2)
	i := strings.SplitN(r[1], ":", 2)
	return r[0], i[0], i[1]
}
