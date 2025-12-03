package integration

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/wizzz/pangolin-ingress-controller/internal/pangolincrd"
)

// Note: These tests require envtest to be set up.
// For now, they serve as documentation and will be enabled when envtest is configured.

func TestLifecycle_IngressUpdated_UpdatesPangolinResource(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US2: Update Exposed Service
	// Given: an Ingress myapp is exposed via Pangolin at app.example.com
	// When: I update the host to newapp.example.com
	// Then: the PangolinResource is updated to reflect the new hostname
}

func TestLifecycle_BackendUpdated_UpdatesPangolinResource(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US2: Update Exposed Service
	// Given: an Ingress points to service frontend:80
	// When: I change the backend to api:8080
	// Then: the PangolinResource target is updated accordingly
}

func TestLifecycle_IngressDeleted_DeletesPangolinResource(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US3: Remove Exposed Service
	// Given: an Ingress is exposed via Pangolin
	// When: I delete the Ingress
	// Then: the corresponding PangolinResource is automatically deleted
}

func TestLifecycle_IngressClassChanged_DeletesPangolinResource(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US3: Remove Exposed Service
	// Given: an Ingress is exposed via Pangolin
	// When: I change its ingressClassName to something else
	// Then: the PangolinResource is removed
}

func TestLifecycle_EnabledFalse_DeletesPangolinResource(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US5: Disable Exposure Temporarily
	// Given: an Ingress is exposed via Pangolin
	// When: I add annotation pangolin.ingress.k8s.io/enabled: "false"
	// Then: the PangolinResource is deleted but the Ingress remains
}

func TestLifecycle_EnabledTrue_CreatesPangolinResource(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US5: Disable Exposure Temporarily
	// Given: an Ingress has enabled: "false" annotation
	// When: I remove the annotation or set it to "true"
	// Then: the PangolinResource is recreated
}

// Test fixtures

func newTestIngress(name, namespace, host string) *networkingv1.Ingress {
	ingressClassName := "pangolin"
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: func() *networkingv1.PathType { p := networkingv1.PathTypePrefix; return &p }(),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newTestTunnel(name string) *pangolincrd.PangolinTunnel {
	return &pangolincrd.PangolinTunnel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: pangolincrd.PangolinTunnelSpec{
			SiteID: "test-site",
		},
	}
}
