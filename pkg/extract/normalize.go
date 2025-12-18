package extract

import (
	"fmt"
	"regexp"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"
)

// olmNamePattern matches OLM-generated names with random suffixes.
// Pattern: {base}-{random} or {base}-op-{random}
// where {random} is a long alphanumeric string (typically 30+ chars).
var olmNamePattern = regexp.MustCompile(`^(.+?)-(op-)?([a-zA-Z0-9]{30,})$`)

// resourceKey uniquely identifies a resource by name and GVK.
type resourceKey struct {
	name string
	gvk  schema.GroupVersionKind
}

// nameMapping tracks the mapping from old OLM-generated names to new normalized names.
type nameMapping struct {
	// oldToNew maps old resource keys to new names
	oldToNew map[resourceKey]string
	// serviceAccountName is the primary service account name (usually the deployment name)
	serviceAccountName string
	// deploymentName is the deployment name
	deploymentName string
}

// normalizeResourceNames normalizes OLM-generated resource names to be simple and consistent.
// It strips random suffixes and generates clean, deterministic names based on the deployment name.
func normalizeResourceNames(objects []runtime.Object) ([]runtime.Object, error) {
	mapping, err := buildNameMapping(objects)
	if err != nil {
		return nil, fmt.Errorf("failed to build name mapping: %w", err)
	}

	return applyNameMapping(objects, mapping)
}

// buildNameMapping analyzes resources and builds a mapping from old to new names.
func buildNameMapping(objects []runtime.Object) (*nameMapping, error) {
	mapping := &nameMapping{
		oldToNew: make(map[resourceKey]string),
	}

	if len(objects) == 0 {
		return mapping, nil
	}

	// Find deployment and service account names
	if err := extractBaseNames(objects, mapping); err != nil {
		return nil, err
	}

	// Build mappings for resources with OLM-generated names
	if err := buildResourceMappings(objects, mapping); err != nil {
		return nil, err
	}

	return mapping, nil
}

// extractBaseNames finds the deployment and service account names from the objects.
func extractBaseNames(objects []runtime.Object, mapping *nameMapping) error {
	for _, obj := range objects {
		metaObj, err := meta.Accessor(obj)
		if err != nil {
			return fmt.Errorf("failed to access object metadata: %w", err)
		}

		gvk := obj.GetObjectKind().GroupVersionKind()

		switch gvk {
		case gvks.Deployment:
			mapping.deploymentName = metaObj.GetName()
		case gvks.ServiceAccount:
			// Use the first service account as the base name
			if mapping.serviceAccountName == "" {
				mapping.serviceAccountName = metaObj.GetName()
			}
		}
	}

	return nil
}

// getBaseName returns the base name to use for normalization.
func getBaseName(mapping *nameMapping) string {
	baseName := mapping.deploymentName
	if baseName == "" {
		baseName = mapping.serviceAccountName
	}
	if baseName == "" {
		baseName = "operator"
	}

	return baseName
}

// RBAC resource types that should have their OLM-generated names normalized.
//
//nolint:gochecknoglobals
var normalizedResourceTypes = sets.New(
	gvks.Role,
	gvks.RoleBinding,
	gvks.ClusterRole,
	gvks.ClusterRoleBinding,
)

// buildResourceMappings creates mappings for all RBAC resources with OLM-generated names.
func buildResourceMappings(objects []runtime.Object, mapping *nameMapping) error {
	baseName := getBaseName(mapping)

	// Track counts per resource type
	counts := make(map[schema.GroupVersionKind]int)

	for _, obj := range objects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if !normalizedResourceTypes.Has(gvk) {
			continue
		}

		metaObj, err := meta.Accessor(obj)
		if err != nil {
			return fmt.Errorf("failed to access object metadata: %w", err)
		}

		name := metaObj.GetName()
		if !isOLMGeneratedName(name) {
			continue
		}

		// Process RBAC resources - suffix is derived from Kind (lowercase)
		suffix := strings.ToLower(gvk.Kind)
		newName := generateResourceName(baseName, suffix, counts[gvk])
		mapping.oldToNew[resourceKey{name: name, gvk: gvk}] = newName
		counts[gvk]++
	}

	return nil
}

// generateResourceName creates a normalized name for a resource.
// If count is 0, returns baseName-suffix. Otherwise, returns baseName-suffix-count.
func generateResourceName(baseName string, suffix string, count int) string {
	if count == 0 {
		return baseName + "-" + suffix
	}

	return baseName + "-" + suffix + "-" + string(rune('0'+count))
}

// isOLMGeneratedName checks if a name matches OLM's generation pattern.
func isOLMGeneratedName(name string) bool {
	return olmNamePattern.MatchString(name)
}

// applyNameMapping applies the name mapping to all resources and their cross-references.
func applyNameMapping(objects []runtime.Object, mapping *nameMapping) ([]runtime.Object, error) {
	if len(mapping.oldToNew) == 0 {
		return objects, nil
	}

	normalized := make([]runtime.Object, 0, len(objects))

	for _, obj := range objects {
		normalizedObj, err := normalizeObject(obj, mapping)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize object: %w", err)
		}
		normalized = append(normalized, normalizedObj)
	}

	return normalized, nil
}

// updateSimpleResourceName updates a resource's name using the metav1.Object interface.
// Works for any resource that only needs name normalization (Role, ClusterRole, ServiceAccount).
func updateSimpleResourceName(obj runtime.Object, mapping *nameMapping) (runtime.Object, error) {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to access object metadata: %w", err)
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	key := resourceKey{name: metaObj.GetName(), gvk: gvk}
	if newName, ok := mapping.oldToNew[key]; ok {
		metaObj.SetName(newName)
	}

	return obj, nil
}

// normalizeObject normalizes a single object's name and cross-references.
func normalizeObject(obj runtime.Object, mapping *nameMapping) (runtime.Object, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	switch gvk {
	// Simple resources - just update name directly with helper
	case gvks.Role, gvks.ClusterRole, gvks.ServiceAccount:
		return updateSimpleResourceName(obj, mapping)

	// Complex resources - keep dedicated functions
	case gvks.RoleBinding:
		return normalizeRoleBinding(obj, mapping)
	case gvks.ClusterRoleBinding:
		return normalizeClusterRoleBinding(obj, mapping)
	case gvks.Deployment:
		return normalizeDeployment(obj, mapping)
	case gvks.ValidatingWebhookConfiguration:
		return normalizeValidatingWebhook(obj, mapping)
	case gvks.MutatingWebhookConfiguration:
		return normalizeMutatingWebhook(obj, mapping)
	}

	return obj, nil
}

// normalizeRoleBinding normalizes a RoleBinding's name and roleRef.
func normalizeRoleBinding(obj runtime.Object, mapping *nameMapping) (runtime.Object, error) {
	rb, err := kube.Convert[*rbacv1.RoleBinding](obj)
	if err != nil {
		return nil, err
	}

	// Update the RoleBinding's own name
	key := resourceKey{name: rb.Name, gvk: gvks.RoleBinding}
	if newName, ok := mapping.oldToNew[key]; ok {
		rb.Name = newName
	}

	// Update the roleRef to point to the normalized Role name
	roleKey := resourceKey{name: rb.RoleRef.Name, gvk: gvks.Role}
	if newRoleName, ok := mapping.oldToNew[roleKey]; ok {
		rb.RoleRef.Name = newRoleName
	}

	return rb, nil
}

// normalizeClusterRoleBinding normalizes a ClusterRoleBinding's name and roleRef.
func normalizeClusterRoleBinding(obj runtime.Object, mapping *nameMapping) (runtime.Object, error) {
	crb, err := kube.Convert[*rbacv1.ClusterRoleBinding](obj)
	if err != nil {
		return nil, err
	}

	// Update the ClusterRoleBinding's own name
	key := resourceKey{name: crb.Name, gvk: gvks.ClusterRoleBinding}
	if newName, ok := mapping.oldToNew[key]; ok {
		crb.Name = newName
	}

	// Update the roleRef to point to the normalized ClusterRole name
	roleKey := resourceKey{name: crb.RoleRef.Name, gvk: gvks.ClusterRole}
	if newClusterRoleName, ok := mapping.oldToNew[roleKey]; ok {
		crb.RoleRef.Name = newClusterRoleName
	}

	return crb, nil
}

// normalizeDeployment normalizes serviceAccountName references in a Deployment.
func normalizeDeployment(obj runtime.Object, mapping *nameMapping) (runtime.Object, error) {
	dep, err := kube.Convert[*appsv1.Deployment](obj)
	if err != nil {
		return nil, err
	}

	// Update serviceAccountName if it was renamed
	saName := dep.Spec.Template.Spec.ServiceAccountName
	if saName != "" {
		saKey := resourceKey{name: saName, gvk: gvks.ServiceAccount}
		if newSAName, ok := mapping.oldToNew[saKey]; ok {
			dep.Spec.Template.Spec.ServiceAccountName = newSAName
		}
	}

	return dep, nil
}

// normalizeValidatingWebhook normalizes webhook configuration names.
func normalizeValidatingWebhook(obj runtime.Object, mapping *nameMapping) (runtime.Object, error) {
	vwc, err := kube.Convert[*admissionregistrationv1.ValidatingWebhookConfiguration](obj)
	if err != nil {
		return nil, err
	}

	vwc.Name = normalizeWebhookName(vwc.Name, mapping.deploymentName, "validating")

	return vwc, nil
}

// normalizeMutatingWebhook normalizes webhook configuration names.
func normalizeMutatingWebhook(obj runtime.Object, mapping *nameMapping) (runtime.Object, error) {
	mwc, err := kube.Convert[*admissionregistrationv1.MutatingWebhookConfiguration](obj)
	if err != nil {
		return nil, err
	}

	mwc.Name = normalizeWebhookName(mwc.Name, mapping.deploymentName, "mutating")

	return mwc, nil
}

// normalizeWebhookName generates a clean webhook name based on deployment name and type.
func normalizeWebhookName(currentName string, deploymentName string, webhookType string) string {
	// If already normalized, keep it
	if !isOLMGeneratedName(currentName) && !strings.Contains(currentName, ".v") {
		return currentName
	}

	if deploymentName == "" {
		deploymentName = "operator"
	}

	return deploymentName + "-" + webhookType + "-webhook"
}
