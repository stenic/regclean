package helpers

import (
	"context"
	"encoding/json"
	"regclean/pkg/utils"
	"strings"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type clusterHelper struct {
	kubeconfig string
}

func NewClusterHelper(kubeconfig string) *clusterHelper {
	return &clusterHelper{
		kubeconfig: kubeconfig,
	}
}

func (h clusterHelper) GetImages(kubeContext string) []string {
	clientset := h.getClientsetForContext(kubeContext)

	images := []string{}
	logrus.Trace("Fetching images from pods")
	images = append(images, h.getPodImages(clientset)...)

	logrus.Trace("Fetching images from statefulsets / daemonsets")
	images = append(images, h.getControllerRevisionImages(clientset)...)

	logrus.Trace("Fetching images from replicasets")
	images = append(images, h.getReplicaSetImages(clientset)...)

	logrus.Trace("Filtering and cleaning images")
	images = utils.Unique(images)
	images = h.cleanImageNames(images)

	return images
}

func (h clusterHelper) getClientsetForContext(context string) *kubernetes.Clientset {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: h.kubeconfig},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()

	if err != nil {
		logrus.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatal(err)
	}

	return clientset
}

func (h clusterHelper) getPodImages(clientset *kubernetes.Clientset) []string {
	images := []string{}
	podList, err := clientset.CoreV1().Pods("").List(context.Background(), v1.ListOptions{})
	if err != nil {
		logrus.Fatal(err)
	}
	for _, pod := range podList.Items {
		for _, container := range pod.Spec.Containers {
			images = append(images, container.Image)
		}
	}
	return images
}

func (h clusterHelper) getControllerRevisionImages(clientset *kubernetes.Clientset) []string {
	images := []string{}
	controllerRevisionList, err := clientset.AppsV1().ControllerRevisions("").List(context.Background(), v1.ListOptions{})
	if err != nil {
		logrus.Fatal(err)
	}
	for _, cr := range controllerRevisionList.Items {
		sts := appsv1.StatefulSet{}
		err := json.Unmarshal(cr.Data.Raw, &sts)
		if err != nil {
			logrus.Fatal(err)
		}

		for _, container := range sts.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}
	return images
}

func (h clusterHelper) getReplicaSetImages(clientset *kubernetes.Clientset) []string {
	images := []string{}
	replicasetList, err := clientset.AppsV1().ReplicaSets("").List(context.Background(), v1.ListOptions{})
	if err != nil {
		logrus.Fatal(err)
	}
	for _, rs := range replicasetList.Items {
		for _, container := range rs.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}
	return images
}

func (h clusterHelper) cleanImageNames(images []string) []string {
	imgs := []string{}
	for _, img := range images {
		img = strings.Split(img, "@")[0]
		imgs = append(imgs, img)
	}

	return imgs
}
