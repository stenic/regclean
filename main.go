package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regclean/pkg/auth"
	"regclean/pkg/helpers"
	"regclean/pkg/ui"
	"regclean/pkg/utils"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/homedir"
	"k8s.io/utils/strings/slices"
)

var (
	kubeContexts       []string
	registryURL        string
	registryUsername   string
	registryPassword   string
	kubeconfig         string
	v                  string
	dryRun             bool
	yolo               bool
	minAge             int
	excludeNameFilters []string
	includeNameFilters []string
	aws                bool
)

var rootCmd = &cobra.Command{
	Use: "regclean",
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

func init() {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := setUpLogs(os.Stdout, v); err != nil {
			return err
		}
		return nil
	}

	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Dry run")
	rootCmd.PersistentFlags().BoolVar(&yolo, "yolo", false, "Don't ask for confirmation")
	rootCmd.PersistentFlags().BoolVar(&aws, "aws", false, "Use AWS credentials for registry")
	rootCmd.PersistentFlags().IntVar(&minAge, "min-age", 30, "Minimum age of images to delete")
	rootCmd.PersistentFlags().StringVarP(&v, "verbosity", "v", logrus.DebugLevel.String(), "Log level (debug, info, warn, error, fatal, panic")
	rootCmd.PersistentFlags().StringVar(&registryURL, "registry-url", os.Getenv("REGCLEAN_REGISTRY_URL"), "URL of the registry you would like to clean")
	rootCmd.PersistentFlags().StringVar(&registryUsername, "registry-username", os.Getenv("REGCLEAN_REGISTRY_USERNAME"), "(optional) credentials")
	rootCmd.PersistentFlags().StringVar(&registryPassword, "registry-password", os.Getenv("REGCLEAN_REGISTRY_PASSWORD"), "(optional) credentials")
	rootCmd.PersistentFlags().StringSliceVar(&kubeContexts, "contexts", strings.Split(os.Getenv("REGCLEAN_CONTEXTS"), ","), "Kubernetes contexts to check for images")
	rootCmd.PersistentFlags().StringSliceVar(&excludeNameFilters, "exclude-name-filters", strings.Split(os.Getenv("REGCLEAN_EXCLUDE_NAME_FILTERS"), ","), "Filters to exclude image names")
	rootCmd.PersistentFlags().StringSliceVar(&includeNameFilters, "include-name-filters", strings.Split(os.Getenv("REGCLEAN_INCLUDE_NAME_FILTERS"), ","), "Filters to include image names")
	if home := homedir.HomeDir(); home != "" {
		rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "(optional) absolute path to the kubeconfig file")
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}

func setUpLogs(out io.Writer, level string) error {
	logrus.SetOutput(out)
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)
	return nil
}

func run() {
	clusterImages := []string{}
	logrus.Info("Fetching images")
	clusterHelper := helpers.NewClusterHelper(kubeconfig)

	for _, kubeContext := range kubeContexts {
		curImages := clusterHelper.GetImages(kubeContext)
		logrus.Debugf("Found %d images in context %s", len(curImages), kubeContext)
		logrus.WithField(
			"images", curImages,
		).Tracef("Found %d images in context %s", len(curImages), kubeContext)
		clusterImages = append(clusterImages, curImages...)
	}
	clusterImages = utils.Unique(clusterImages)
	logrus.Infof("Collected %d unique images in %d contexts", len(clusterImages), len(kubeContexts))

	logrus.Info("Fetching images from registry")
	if aws {
		registryUsername, registryPassword = auth.GetAWSCredentials()
	}

	regHelper := helpers.NewRegHelper(registryURL, registryUsername, registryPassword, dryRun)
	registryImages := regHelper.GetImages()
	logrus.Infof("Collected %d images from registry", len(registryImages))
	logrus.WithField(
		"images", registryImages,
	).Tracef("Found %d images in registry", len(registryImages))

	toDelete := []string{}
	toKeep := []string{}
	for _, registryImage := range registryImages {
		if !slices.Contains(clusterImages, registryImage) {
			toDelete = append(toDelete, registryImage)
		} else {
			toKeep = append(toKeep, registryImage)
		}
	}

	filterHelper := helpers.NewFilterHelper(*regHelper)
	filterHelper.MinAge = minAge
	filterHelper.ExcludeNameFilters = utils.DeleteEmpty(excludeNameFilters)
	filterHelper.IncludeNameFilters = utils.DeleteEmpty(includeNameFilters)
	toDelete, filterCount := filterHelper.FilterImages(toDelete)

	total := uint64(0)
	for _, image := range toDelete {
		size, _ := regHelper.GetImageSize(image)
		total += size
	}

	for _, image := range toDelete {
		img := strings.TrimPrefix(image, regHelper.RegPrefix+"/")
		created, _ := regHelper.GetImageDate(image)
		size, _ := regHelper.GetImageSize(image)

		logrus.WithFields(logrus.Fields{
			"created": created.Format(time.DateTime),
			"size":    humanize.Bytes(size),
		}).Debugf("Deleting %s", img)
	}

	logrus.Infof("Found %d images to delete (%s) and %d to keep", len(toDelete), humanize.Bytes(total), len(toKeep)+filterCount)

	if len(toDelete) == 0 {
		logrus.Info("Nothing to delete")
		return
	}

	if yolo && !ui.YesNo("We will delete all without asking, are you sure?") {
		logrus.Fatal("Back to safety")
	}

	for _, image := range toDelete {
		if yolo || ui.YesNo(fmt.Sprintf("Delete %s?", image)) {
			if err := regHelper.DeleteImage(image); err != nil {
				logrus.Errorf("Failed to delete image: %s", err)
			}
		}
	}
}
