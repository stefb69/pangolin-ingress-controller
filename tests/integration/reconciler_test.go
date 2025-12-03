package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/wizzz/pangolin-ingress-controller/internal/pangolincrd"
)

// Note: These tests require envtest to be set up.
// For now, they serve as documentation and will be enabled when envtest is configured.

func TestReconciler_IngressCreated_CreatesPangolinResource(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US1: Expose Service via Ingress
	// Given: pangolin-operator is installed with a working tunnel
	// When: I create an Ingress with ingressClassName: pangolin
	// Then: a corresponding PangolinResource is created

	ctx := context.Background()
	_ = ctx // Will be used with envtest client

	// Create test tunnel
	tunnel := &pangolincrd.PangolinTunnel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}
	_ = tunnel

	// Create test ingress
	ingressClassName := "pangolin"
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: "app.example.com",
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
	_ = ingress

	// Verify PangolinResource is created
	// var resource pangolincrd.PangolinResource
	// Eventually the resource should exist with correct spec
}

func TestReconciler_TunnelNotFound_EmitsWarningEvent(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US1 edge case: missing tunnel
	// Given: no tunnel is configured
	// When: I create an Ingress with ingressClassName: pangolin
	// Then: I receive a clear warning event explaining the tunnel is missing
}

func TestReconciler_OwnerReferenceSetCorrectly(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies garbage collection setup
	// Given: an Ingress exists with ingressClassName: pangolin
	// When: I check the cluster resources
	// Then: I see a PangolinResource owned by that Ingress
}

func TestReconciler_AnnotationOverridesApplied(t *testing.T) {
	t.Skip("Requires envtest setup")

	// This test verifies US4: Override Domain Configuration
	// Given: an Ingress with host internal.corp.example.com
	// When: I add annotation pangolin.ingress.k8s.io/subdomain: myapp
	// Then: the PangolinResource uses subdomain myapp instead of derived value
}

// Helper functions for integration tests

func waitForPangolinResource(
	ctx context.Context,
	c client.Client,
	name types.NamespacedName,
	timeout time.Duration,
) (*pangolincrd.PangolinResource, error) {
	var resource pangolincrd.PangolinResource
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if err := c.Get(ctx, name, &resource); err == nil {
			return &resource, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil, context.DeadlineExceeded
}

func assertPangolinResourceSpec(t *testing.T, resource *pangolincrd.PangolinResource, expected pangolincrd.PangolinResourceSpec) {
	assert.Equal(t, expected.Enabled, resource.Spec.Enabled)
	assert.Equal(t, expected.TunnelRef.Name, resource.Spec.TunnelRef.Name)

	if expected.HTTPConfig != nil {
		require.NotNil(t, resource.Spec.HTTPConfig)
		assert.Equal(t, expected.HTTPConfig.DomainName, resource.Spec.HTTPConfig.DomainName)
		assert.Equal(t, expected.HTTPConfig.Subdomain, resource.Spec.HTTPConfig.Subdomain)
	}

	if len(expected.Targets) > 0 {
		require.NotEmpty(t, resource.Spec.Targets)
		assert.Equal(t, expected.Targets[0].IP, resource.Spec.Targets[0].IP)
		assert.Equal(t, expected.Targets[0].Port, resource.Spec.Targets[0].Port)
	}
}
