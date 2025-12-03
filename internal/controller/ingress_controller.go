// Package controller implements the Ingress reconciliation logic.
package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/wizzz/pangolin-ingress-controller/internal/config"
	"github.com/wizzz/pangolin-ingress-controller/internal/pangolincrd"
	"github.com/wizzz/pangolin-ingress-controller/internal/util"
)

const (
	// AnnotationEnabled controls whether the Ingress is managed by PIC.
	AnnotationEnabled = "pangolin.ingress.k8s.io/enabled"

	// AnnotationTunnelName overrides the tunnel name.
	AnnotationTunnelName = "pangolin.ingress.k8s.io/tunnel-name"

	// AnnotationDomainName overrides the domain name.
	AnnotationDomainName = "pangolin.ingress.k8s.io/domain-name"

	// AnnotationSubdomain overrides the subdomain.
	AnnotationSubdomain = "pangolin.ingress.k8s.io/subdomain"

	// AnnotationSSO enables SSO authentication.
	AnnotationSSO = "pangolin.ingress.k8s.io/sso"

	// AnnotationBlockAccess blocks access until authenticated.
	AnnotationBlockAccess = "pangolin.ingress.k8s.io/block-access"

	// LabelIngressUID identifies the source Ingress.
	LabelIngressUID = "pic.ingress.k8s.io/uid"

	// LabelIngressName identifies the source Ingress name.
	LabelIngressName = "pic.ingress.k8s.io/name"

	// LabelIngressNamespace identifies the source Ingress namespace.
	LabelIngressNamespace = "pic.ingress.k8s.io/namespace"

	// IngressClassPangolin is the default ingress class name.
	IngressClassPangolin = "pangolin"

	// IngressClassPrefix is the prefix for multi-tunnel ingress classes.
	IngressClassPrefix = "pangolin-"
)

// HostPathGroup groups all HTTP paths for a single host.
// Used internally during multi-host reconciliation to aggregate paths
// from potentially multiple rules that share the same host.
type HostPathGroup struct {
	Host  string
	Paths []networkingv1.HTTPIngressPath
}

// IngressReconciler reconciles Ingress resources.
type IngressReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Config   *config.Config
	Log      logr.Logger
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolintunnels,verbs=get;list;watch
// +kubebuilder:rbac:groups=tunnel.pangolin.io,resources=pangolinresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles Ingress changes.
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("ingress", req.NamespacedName)
	log.V(1).Info("Reconciling Ingress")

	// Fetch the Ingress
	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if errors.IsNotFound(err) {
			// Ingress deleted - PangolinResource will be garbage collected via ownerReference
			log.V(1).Info("Ingress not found, assuming deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Ingress")
		return ctrl.Result{}, err
	}

	// Check if this Ingress is managed by PIC
	if !r.isManaged(&ingress) {
		log.V(1).Info("Ingress not managed by PIC")
		// If we previously managed it, delete the PangolinResource
		return r.handleUnmanaged(ctx, &ingress)
	}

	// Resolve tunnel name
	tunnelName, err := r.resolveTunnel(&ingress)
	if err != nil {
		log.Error(err, "Failed to resolve tunnel")
		r.Recorder.Event(&ingress, corev1.EventTypeWarning, "TunnelResolutionFailed", err.Error())
		return ctrl.Result{}, err
	}

	// Validate tunnel exists and get its namespace
	tunnelNamespace, err := r.validateTunnel(ctx, tunnelName)
	if err != nil {
		log.Error(err, "Tunnel validation failed", "tunnel", tunnelName)
		r.Recorder.Event(&ingress, corev1.EventTypeWarning, "TunnelNotFound",
			fmt.Sprintf("Tunnel %q not found", tunnelName))
		// Requeue to retry
		return ctrl.Result{Requeue: true}, nil
	}

	// Process all hosts in the Ingress
	return r.processHosts(ctx, &ingress, tunnelName, tunnelNamespace)
}

// processHosts processes all hosts in an Ingress, creating/updating PangolinResources.
// It handles multi-host Ingresses by creating one PangolinResource per unique host.
func (r *IngressReconciler) processHosts(
	ctx context.Context,
	ingress *networkingv1.Ingress,
	tunnelName string,
	tunnelNamespace string,
) (ctrl.Result, error) {
	log := r.Log.WithValues("ingress", types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace})

	// Check for empty rules
	if len(ingress.Spec.Rules) == 0 {
		log.Info("Ingress has no rules, skipping")
		r.Recorder.Event(ingress, corev1.EventTypeWarning, "NoRules", "Ingress has no rules defined")
		return ctrl.Result{}, nil
	}

	// Collect and deduplicate hosts with their paths
	hostGroups := r.collectHostPaths(ingress)

	if len(hostGroups) == 0 {
		log.Info("Ingress has no valid hosts after filtering")
		return ctrl.Result{}, nil
	}

	// Track which PangolinResource names we create/update for orphan cleanup
	desiredNames := make(map[string]bool)

	// Process each host
	for _, group := range hostGroups {
		// Build desired PangolinResource for this host
		desired, err := r.buildDesiredPangolinResource(ingress, group.Host, group.Paths, tunnelName, tunnelNamespace)
		if err != nil {
			log.Error(err, "Failed to build desired PangolinResource", "host", group.Host)
			r.Recorder.Event(ingress, corev1.EventTypeWarning, "InvalidHost",
				fmt.Sprintf("Host %q: %s", group.Host, err.Error()))
			continue // Skip this host, try others
		}

		// Set owner reference for garbage collection
		if err := ctrl.SetControllerReference(ingress, desired, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference", "host", group.Host)
			return ctrl.Result{}, err
		}

		desiredNames[desired.Name] = true

		// Create or update PangolinResource
		if _, err := r.reconcilePangolinResource(ctx, ingress, desired); err != nil {
			log.Error(err, "Failed to reconcile PangolinResource", "host", group.Host)
			return ctrl.Result{}, err
		}
	}

	// Clean up orphaned PangolinResources (hosts removed from Ingress)
	if err := r.cleanupOrphanedResources(ctx, ingress, desiredNames); err != nil {
		log.Error(err, "Failed to cleanup orphaned resources")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// collectHostPaths groups all paths by host, deduplicating hosts that appear in multiple rules.
// Empty hosts are skipped with a warning event.
func (r *IngressReconciler) collectHostPaths(ingress *networkingv1.Ingress) []HostPathGroup {
	hostMap := make(map[string][]networkingv1.HTTPIngressPath)

	for _, rule := range ingress.Spec.Rules {
		// Skip empty hosts (FR-007)
		if rule.Host == "" {
			r.Recorder.Event(ingress, corev1.EventTypeWarning, "EmptyHost",
				"Rule with empty host skipped")
			continue
		}

		// Collect paths for this host
		if rule.HTTP != nil {
			hostMap[rule.Host] = append(hostMap[rule.Host], rule.HTTP.Paths...)
		}
	}

	// Convert map to slice for deterministic ordering
	var groups []HostPathGroup
	for host, paths := range hostMap {
		groups = append(groups, HostPathGroup{
			Host:  host,
			Paths: paths,
		})
	}

	return groups
}

// cleanupOrphanedResources deletes PangolinResources that are no longer needed.
// This happens when a host is removed from an Ingress.
func (r *IngressReconciler) cleanupOrphanedResources(
	ctx context.Context,
	ingress *networkingv1.Ingress,
	desiredNames map[string]bool,
) error {
	log := r.Log.WithValues("ingress", types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace})

	// List all PangolinResources owned by this Ingress
	var resourceList pangolincrd.PangolinResourceList
	if err := r.List(ctx, &resourceList,
		client.InNamespace(ingress.Namespace),
		client.MatchingLabels{LabelIngressUID: string(ingress.UID)},
	); err != nil {
		return fmt.Errorf("failed to list PangolinResources: %w", err)
	}

	// Delete resources that are no longer desired
	for _, resource := range resourceList.Items {
		if !desiredNames[resource.Name] {
			log.Info("Deleting orphaned PangolinResource", "resource", resource.Name)
			if err := r.Delete(ctx, &resource); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to delete orphaned resource %s: %w", resource.Name, err)
			}
			r.Recorder.Event(ingress, corev1.EventTypeNormal, "Deleted",
				fmt.Sprintf("Deleted PangolinResource %s (host removed)", resource.Name))
		}
	}

	return nil
}

// isManaged checks if the Ingress should be managed by PIC.
func (r *IngressReconciler) isManaged(ingress *networkingv1.Ingress) bool {
	// Check enabled annotation
	if enabled, ok := ingress.Annotations[AnnotationEnabled]; ok {
		if strings.ToLower(enabled) == "false" {
			return false
		}
	}

	// Check ingressClassName
	className := ""
	if ingress.Spec.IngressClassName != nil {
		className = *ingress.Spec.IngressClassName
	}

	return className == IngressClassPangolin || strings.HasPrefix(className, IngressClassPrefix)
}

// resolveTunnel determines the tunnel name from the Ingress.
func (r *IngressReconciler) resolveTunnel(ingress *networkingv1.Ingress) (string, error) {
	// Check annotation override
	if tunnelName, ok := ingress.Annotations[AnnotationTunnelName]; ok && tunnelName != "" {
		return tunnelName, nil
	}

	// Get ingress class
	className := ""
	if ingress.Spec.IngressClassName != nil {
		className = *ingress.Spec.IngressClassName
	}

	// Default class -> default tunnel
	if className == IngressClassPangolin {
		return r.Config.DefaultTunnelName, nil
	}

	// Multi-tunnel class -> lookup in mapping
	if strings.HasPrefix(className, IngressClassPrefix) {
		suffix := strings.TrimPrefix(className, IngressClassPrefix)
		if tunnelName, ok := r.Config.TunnelMapping[suffix]; ok {
			return tunnelName, nil
		}
		// If no mapping, use suffix as tunnel name
		return suffix, nil
	}

	return "", fmt.Errorf("cannot resolve tunnel for ingressClassName %q", className)
}

// validateTunnel checks if the PangolinTunnel exists in any namespace.
// Returns the namespace where the tunnel was found.
func (r *IngressReconciler) validateTunnel(ctx context.Context, tunnelName string) (string, error) {
	var tunnelList pangolincrd.PangolinTunnelList
	if err := r.List(ctx, &tunnelList); err != nil {
		return "", fmt.Errorf("failed to list tunnels: %w", err)
	}

	for _, tunnel := range tunnelList.Items {
		if tunnel.Name == tunnelName {
			return tunnel.Namespace, nil
		}
	}

	return "", fmt.Errorf("tunnel %q not found", tunnelName)
}

// buildDesiredPangolinResource creates the desired PangolinResource spec.
// It accepts the host and its associated paths (already collected and deduplicated).
func (r *IngressReconciler) buildDesiredPangolinResource(
	ingress *networkingv1.Ingress,
	host string,
	paths []networkingv1.HTTPIngressPath,
	tunnelName string,
	tunnelNamespace string,
) (*pangolincrd.PangolinResource, error) {
	// Split host into subdomain and domain
	subdomain, domain, err := util.SplitHost(host)
	if err != nil {
		return nil, fmt.Errorf("invalid host %q: %w", host, err)
	}

	// Apply annotation overrides
	if override, ok := ingress.Annotations[AnnotationDomainName]; ok && override != "" {
		domain = override
	}
	if override, ok := ingress.Annotations[AnnotationSubdomain]; ok && override != "" {
		subdomain = override
	}

	// Build targets from the provided paths for this host
	var targets []pangolincrd.Target

	for _, path := range paths {
		if path.Backend.Service == nil {
			continue
		}

		backendHost := fmt.Sprintf("%s.%s.svc.cluster.local",
			path.Backend.Service.Name, ingress.Namespace)
		var backendPort int32
		if path.Backend.Service.Port.Number != 0 {
			backendPort = path.Backend.Service.Port.Number
		}

		// Map Ingress pathType to Pangolin pathMatchType
		pathMatchType := "prefix" // default
		if path.PathType != nil {
			switch *path.PathType {
			case networkingv1.PathTypeExact:
				pathMatchType = "exact"
			case networkingv1.PathTypePrefix:
				pathMatchType = "prefix"
			case networkingv1.PathTypeImplementationSpecific:
				pathMatchType = "prefix" // default to prefix
			}
		}

		// Calculate priority based on path specificity
		// Longer paths get higher priority (matched first)
		priority := int32(100 + len(path.Path)*10)
		if priority > 1000 {
			priority = 1000
		}

		target := pangolincrd.Target{
			IP:            backendHost,
			Port:          backendPort,
			Method:        r.Config.BackendScheme,
			Path:          path.Path,
			PathMatchType: pathMatchType,
			Priority:      priority,
		}
		targets = append(targets, target)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no valid backends found in Ingress paths")
	}

	// Generate deterministic name
	name := util.GenerateName(ingress.Namespace, ingress.Name, host)

	// Generate display name for Pangolin UI
	// Must be unique per host since Pangolin uses this as resource identifier
	displayName := fmt.Sprintf("%s/%s/%s", ingress.Namespace, ingress.Name, host)

	// Protocol is always "http" for Ingress resources
	// Pangolin handles TLS termination automatically
	// The TLS configuration in Ingress is informational only
	protocol := "http"
	_ = ingress.Spec.TLS // TLS config available for future use if needed

	// Parse authentication annotations
	// By default: SSO disabled, access allowed (no blocking)
	ssoEnabled := ingress.Annotations[AnnotationSSO] == "true"
	blockAccess := ingress.Annotations[AnnotationBlockAccess] == "true"

	return &pangolincrd.PangolinResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ingress.Namespace,
			Labels: map[string]string{
				LabelIngressUID:       string(ingress.UID),
				LabelIngressName:      ingress.Name,
				LabelIngressNamespace: ingress.Namespace,
			},
		},
		Spec: pangolincrd.PangolinResourceSpec{
			Name:     displayName,
			Enabled:  true,
			Protocol: protocol,
			TunnelRef: pangolincrd.TunnelRef{
				Name:      tunnelName,
				Namespace: tunnelNamespace,
			},
			HTTPConfig: &pangolincrd.HTTPConfig{
				DomainName:  domain,
				Subdomain:   subdomain,
				SSO:         ssoEnabled,
				BlockAccess: blockAccess,
			},
			Targets: targets,
		},
	}, nil
}

// reconcilePangolinResource creates or updates the PangolinResource.
func (r *IngressReconciler) reconcilePangolinResource(
	ctx context.Context,
	ingress *networkingv1.Ingress,
	desired *pangolincrd.PangolinResource,
) (ctrl.Result, error) {
	log := r.Log.WithValues(
		"ingress", types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace},
		"pangolinresource", desired.Name,
	)

	// Check if exists
	var existing pangolincrd.PangolinResource
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, &existing)

	if errors.IsNotFound(err) {
		// Create new resource
		log.Info("Creating PangolinResource")
		if err := r.Create(ctx, desired); err != nil {
			log.Error(err, "Failed to create PangolinResource")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(ingress, corev1.EventTypeNormal, "Created",
			fmt.Sprintf("Created PangolinResource %s", desired.Name))
		return ctrl.Result{}, nil
	}

	if err != nil {
		log.Error(err, "Failed to get existing PangolinResource")
		return ctrl.Result{}, err
	}

	// Update if changed
	if r.specChanged(&existing.Spec, &desired.Spec) {
		log.Info("Updating PangolinResource")
		existing.Spec = desired.Spec
		if err := r.Update(ctx, &existing); err != nil {
			log.Error(err, "Failed to update PangolinResource")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(ingress, corev1.EventTypeNormal, "Updated",
			fmt.Sprintf("Updated PangolinResource %s", desired.Name))
	}

	return ctrl.Result{}, nil
}

// specChanged compares two PangolinResourceSpecs.
func (r *IngressReconciler) specChanged(current, desired *pangolincrd.PangolinResourceSpec) bool {
	if current.Name != desired.Name {
		return true
	}
	if current.Enabled != desired.Enabled {
		return true
	}
	if current.Protocol != desired.Protocol {
		return true
	}
	if current.TunnelRef.Name != desired.TunnelRef.Name {
		return true
	}
	if current.TunnelRef.Namespace != desired.TunnelRef.Namespace {
		return true
	}
	if current.HTTPConfig != nil && desired.HTTPConfig != nil {
		if current.HTTPConfig.DomainName != desired.HTTPConfig.DomainName {
			return true
		}
		if current.HTTPConfig.Subdomain != desired.HTTPConfig.Subdomain {
			return true
		}
		if current.HTTPConfig.SSO != desired.HTTPConfig.SSO {
			return true
		}
		if current.HTTPConfig.BlockAccess != desired.HTTPConfig.BlockAccess {
			return true
		}
	}
	// Compare targets arrays
	if len(current.Targets) != len(desired.Targets) {
		return true
	}
	for i := range current.Targets {
		if i >= len(desired.Targets) {
			return true
		}
		ct := current.Targets[i]
		dt := desired.Targets[i]
		if ct.IP != dt.IP || ct.Port != dt.Port || ct.Method != dt.Method ||
			ct.Path != dt.Path || ct.PathMatchType != dt.PathMatchType || ct.Priority != dt.Priority {
			return true
		}
	}
	return false
}

// handleUnmanaged removes PangolinResource for unmanaged Ingress.
func (r *IngressReconciler) handleUnmanaged(ctx context.Context, ingress *networkingv1.Ingress) (ctrl.Result, error) {
	log := r.Log.WithValues("ingress", types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace})

	// Find any PangolinResources owned by this Ingress
	var resourceList pangolincrd.PangolinResourceList
	if err := r.List(ctx, &resourceList,
		client.InNamespace(ingress.Namespace),
		client.MatchingLabels{LabelIngressUID: string(ingress.UID)},
	); err != nil {
		log.Error(err, "Failed to list PangolinResources")
		return ctrl.Result{}, err
	}

	for _, resource := range resourceList.Items {
		log.Info("Deleting PangolinResource for unmanaged Ingress", "resource", resource.Name)
		if err := r.Delete(ctx, &resource); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete PangolinResource")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(ingress, corev1.EventTypeNormal, "Deleted",
			fmt.Sprintf("Deleted PangolinResource %s (Ingress no longer managed)", resource.Name))
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Owns(&pangolincrd.PangolinResource{}).
		Complete(r)
}

// NewIngressReconciler creates a new IngressReconciler.
func NewIngressReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	cfg *config.Config,
	log logr.Logger,
	recorder record.EventRecorder,
) *IngressReconciler {
	return &IngressReconciler{
		Client:   client,
		Scheme:   scheme,
		Config:   cfg,
		Log:      log,
		Recorder: recorder,
	}
}
