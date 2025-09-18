package k8s

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestK8sClientRealCluster tests real GKE cluster connection
func TestK8sClientRealCluster(t *testing.T) {
	// Skip this test unless explicitly running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Create K8s client
	client, err := NewK8sClient()
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	// Test getting deployments and pods
	ctx := context.Background()
	deployments, err := client.GetDeploymentAndPods(ctx)
	if err != nil {
		t.Fatalf("Failed to get deployments and pods: %v", err)
	}

	// Print results
	fmt.Printf("Found %d deployments:\n", len(deployments))
	for i, deployment := range deployments {
		fmt.Printf("Deployment %d:\n", i+1)
		fmt.Printf("  Name: %s\n", deployment.Name)
		fmt.Printf("  Namespace: %s\n", deployment.Namespace)
		fmt.Printf("  Replicas: %d\n", deployment.Replicas)
		fmt.Printf("  Ready: %d\n", deployment.Ready)
		fmt.Printf("  Labels: %v\n", deployment.Labels)
		fmt.Printf("  Pods Count: %d\n", len(deployment.Pods))

		for j, pod := range deployment.Pods {
			fmt.Printf("    Pod %d:\n", j+1)
			fmt.Printf("      Name: %s\n", pod.Name)
			fmt.Printf("      Status: %s\n", pod.Status)
			fmt.Printf("      Ready: %t\n", pod.Ready)
			fmt.Printf("      Node: %s\n", pod.NodeName)
			fmt.Printf("      IP: %s\n", pod.IP)
		}
		fmt.Println()
	}

	// Basic validation
	if len(deployments) == 0 {
		t.Log("Warning: No deployments found")
	} else {
		t.Logf("Successfully retrieved %d deployments", len(deployments))
	}
}

// TestK8sClientConnection tests K8s connection
func TestK8sClientConnection(t *testing.T) {
	// Skip this test unless explicitly running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Test creating K8s client
	client, err := NewK8sClient()
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	// Test basic connection - try to list all namespaces
	ctx := context.Background()
	namespaces, err := client.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to connect to K8s cluster: %v", err)
	}

	fmt.Printf("Successfully connected to K8s cluster, found %d namespaces:\n", len(namespaces.Items))
	for _, ns := range namespaces.Items {
		fmt.Printf("  - %s\n", ns.Name)
	}

	t.Log("K8s connection test successful")
}

// TestIsPodReady tests isPodReady function
func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name     string
		pod      corev1.Pod
		expected bool
	}{
		{
			name: "Pod is ready",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Pod is not ready",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Pod has no ready condition",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Pod has no conditions",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodReady(tt.pod)
			if result != tt.expected {
				t.Errorf("isPodReady() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetDeploymentAndPodsWithOptions tests the option pattern functionality
func TestGetDeploymentAndPodsWithOptions(t *testing.T) {
	// Skip this test unless explicitly running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Create K8s client
	client, err := NewK8sClient()
	if err != nil {
		t.Fatalf("Failed to create K8s client: %v", err)
	}

	ctx := context.Background()

	// Test 1: Get deployments from specific namespaces
	t.Run("WithNamespaces", func(t *testing.T) {
		deployments, err := client.GetDeploymentAndPods(ctx, WithNamespaces("meepo-dev", "meepo-stg"))
		if err != nil {
			t.Fatalf("Failed to get deployments with namespaces: %v", err)
		}

		fmt.Printf("Found %d deployments in meepo-dev and meepo-stg namespaces:\n", len(deployments))
		for _, deployment := range deployments {
			fmt.Printf("  - %s/%s (Replicas: %d, Ready: %d)\n",
				deployment.Namespace, deployment.Name, deployment.Replicas, deployment.Ready)
		}

		// Verify all deployments are from the specified namespaces
		for _, deployment := range deployments {
			if deployment.Namespace != "meepo-dev" && deployment.Namespace != "meepo-stg" {
				t.Errorf("Expected deployment from meepo-dev or meepo-stg, got %s", deployment.Namespace)
			}
		}
	})

	// Test 2: Get deployments with label selector
	t.Run("WithLabels", func(t *testing.T) {
		deployments, err := client.GetDeploymentAndPods(ctx, WithLabels(map[string]string{"app": "game"}))
		if err != nil {
			t.Fatalf("Failed to get deployments with labels: %v", err)
		}

		fmt.Printf("Found %d deployments with app=game label:\n", len(deployments))
		for _, deployment := range deployments {
			fmt.Printf("  - %s/%s (Labels: %v)\n",
				deployment.Namespace, deployment.Name, deployment.Labels)
		}

		// Verify all deployments have the game label
		for _, deployment := range deployments {
			if deployment.Labels["app"] != "game" {
				t.Errorf("Expected deployment with app=game label, got %v", deployment.Labels)
			}
		}
	})

	// Test 3: Combine multiple options
	t.Run("WithMultipleOptions", func(t *testing.T) {
		deployments, err := client.GetDeploymentAndPods(ctx,
			WithNamespaces("meepo-dev"),
			WithLabels(map[string]string{"app": "payment"}))
		if err != nil {
			t.Fatalf("Failed to get deployments with multiple options: %v", err)
		}

		fmt.Printf("Found %d payment deployments in meepo-dev:\n", len(deployments))
		for _, deployment := range deployments {
			fmt.Printf("  - %s/%s (Labels: %v)\n",
				deployment.Namespace, deployment.Name, deployment.Labels)
		}

		// Verify all conditions are met
		for _, deployment := range deployments {
			if deployment.Namespace != "meepo-dev" {
				t.Errorf("Expected deployment from meepo-dev, got %s", deployment.Namespace)
			}
			if deployment.Labels["app"] != "payment" {
				t.Errorf("Expected deployment with app=payment label, got %v", deployment.Labels)
			}
		}
	})

	// Test 4: No options (should get all deployments)
	t.Run("NoOptions", func(t *testing.T) {
		deployments, err := client.GetDeploymentAndPods(ctx)
		if err != nil {
			t.Fatalf("Failed to get all deployments: %v", err)
		}

		fmt.Printf("Found %d total deployments (no filters):\n", len(deployments))
		if len(deployments) == 0 {
			t.Log("Warning: No deployments found")
		} else {
			t.Logf("Successfully retrieved %d deployments", len(deployments))
		}
	})
}
