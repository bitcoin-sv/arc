package k8s_client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8sClient struct {
	client *kubernetes.Clientset
}

func New() (*K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8sClient{client: clientSet}, nil
}

func (k *K8sClient) GetPodNames(ctx context.Context, namespace string) (map[string]struct{}, error) {
	activePodsK8s, err := k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	podNames := map[string]struct{}{}
	for _, item := range activePodsK8s.Items {
		podNames[item.Name] = struct{}{}
	}

	return podNames, nil
}
