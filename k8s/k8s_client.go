package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type DeploymentInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Replicas  int32             `json:"replicas"`
	Ready     int32             `json:"ready"`
	Labels    map[string]string `json:"labels"`
	Pods      []PodInfo         `json:"pods"`
}

type PodInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Status    string            `json:"status"`
	Ready     bool              `json:"ready"`
	Labels    map[string]string `json:"labels"`
	NodeName  string            `json:"node_name"`
	IP        string            `json:"ip"`
}

type K8sClient struct {
	client *kubernetes.Clientset
}

func NewK8sClient() (*K8sClient, error) {
	var config *rest.Config
	var err error

	config, err = rest.InClusterConfig()
	if err != nil {
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get k8s config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return &K8sClient{
		client: clientset,
	}, nil
}

// GetDeploymentOptions defines options for GetDeploymentAndPods
type GetDeploymentOptions struct {
	Namespaces []string
	Labels     map[string]string
}

// GetDeploymentOption defines a function that configures GetDeploymentOptions
type GetDeploymentOption func(*GetDeploymentOptions)

// WithNamespaces sets the namespaces to filter deployments
func WithNamespaces(namespaces ...string) GetDeploymentOption {
	return func(opts *GetDeploymentOptions) {
		opts.Namespaces = namespaces
	}
}

// WithLabels sets the label selector to filter deployments
func WithLabels(labels map[string]string) GetDeploymentOption {
	return func(opts *GetDeploymentOptions) {
		opts.Labels = labels
	}
}

func (k *K8sClient) GetDeploymentAndPods(ctx context.Context, options ...GetDeploymentOption) ([]DeploymentInfo, error) {
	// Apply default options
	opts := &GetDeploymentOptions{}
	for _, option := range options {
		option(opts)
	}

	var allDeployments []appsv1.Deployment

	// Build label selector
	var labelSelector string
	if len(opts.Labels) > 0 {
		var selectors []string
		for key, value := range opts.Labels {
			selectors = append(selectors, fmt.Sprintf("%s=%s", key, value))
		}
		labelSelector = strings.Join(selectors, ",")
	}

	// If no namespaces specified, get all deployments
	if len(opts.Namespaces) == 0 {
		deployments, err := k.client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list deployments: %w", err)
		}
		allDeployments = deployments.Items
	} else {
		// Get deployments from specified namespaces
		for _, namespace := range opts.Namespaces {
			deployments, err := k.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				continue
			}
			allDeployments = append(allDeployments, deployments.Items...)
		}
	}

	deploymentInfos := lo.Map(allDeployments, func(deployment appsv1.Deployment, _ int) DeploymentInfo {
		pods, err := k.getPodsForDeployment(ctx, deployment)
		if err != nil {
			return DeploymentInfo{}
		}

		return DeploymentInfo{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
			Replicas:  *deployment.Spec.Replicas,
			Ready:     deployment.Status.ReadyReplicas,
			Labels:    deployment.Labels,
			Pods:      pods,
		}
	})

	return deploymentInfos, nil
}

func (k *K8sClient) getPodsForDeployment(ctx context.Context, deployment appsv1.Deployment) ([]PodInfo, error) {
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("failed to get selector for deployment: %w", err)
	}

	pods, err := k.client.CoreV1().Pods(deployment.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for deployment: %w", err)
	}

	podInfos := lo.Map(pods.Items, func(pod corev1.Pod, _ int) PodInfo {
		return PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Ready:     isPodReady(pod),
			Labels:    pod.Labels,
			NodeName:  pod.Spec.NodeName,
			IP:        pod.Status.PodIP,
		}
	})

	return podInfos, nil
}

func isPodReady(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
