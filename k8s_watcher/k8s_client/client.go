package k8s_client

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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

func (k *K8sClient) GetPodWatcher(ctx context.Context, namespace string, podName string) (watch.Interface, error) {
	watcher, err := k.client.CoreV1().Events(namespace).Watch(ctx, metav1.ListOptions{
		TypeMeta:      metav1.TypeMeta{},
		LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", podName),
	})

	if err != nil {
		return nil, err
	}

	return watcher, nil
}

func (k *K8sClient) GetPods(ctx context.Context, namespace string) (*v1.PodList, error) {
	return k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
}
