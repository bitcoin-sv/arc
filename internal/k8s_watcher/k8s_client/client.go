package k8s_client

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"

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

func (k *K8sClient) GetRunningPodNames(ctx context.Context, namespace string, podName string) (map[string]struct{}, error) {
	pods, err := k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", podName),
	})

	if err != nil {
		return nil, err
	}

	podNames := map[string]struct{}{}
	for _, item := range pods.Items {
		if item.Status.Phase == v1.PodRunning {
			podNames[item.Name] = struct{}{}
		}
	}

	return podNames, nil
}

func (k *K8sClient) GetRunningPodNamesSlice(ctx context.Context, namespace string, podName string) ([]string, error) {
	pods, err := k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/instance=%s", podName),
	})

	if err != nil {
		return nil, err
	}
	podNames := make([]string, len(pods.Items))
	for i, item := range pods.Items {
		if item.Status.Phase == v1.PodRunning {
			podNames[i] = item.Name
		}
	}

	return podNames, nil
}
