package extract

import (
	"regexp"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// olmNamePattern matches OLM-generated names with random suffixes.
// Pattern: {base}-{random} or {base}-op-{random}
// where {random} is a long alphanumeric string (typically 30+ chars).
var olmNamePattern = regexp.MustCompile(`^(.+?)-(op-)?([a-zA-Z0-9]{30,})$`)

// resourceKey uniquely identifies a resource by name and kind.
type resourceKey struct {
	name string
	kind string
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
func normalizeResourceNames(objects []runtime.Object) []runtime.Object {
	mapping := buildNameMapping(objects)

	return applyNameMapping(objects, mapping)
}

// buildNameMapping analyzes resources and builds a mapping from old to new names.
func buildNameMapping(objects []runtime.Object) *nameMapping {
	mapping := &nameMapping{
		oldToNew: make(map[resourceKey]string),
	}

	if len(objects) == 0 {
		return mapping
	}

	// Find deployment and service account names
	extractBaseNames(objects, mapping)

	// Build mappings for resources with OLM-generated names
	buildResourceMappings(objects, mapping)

	return mapping
}

// extractBaseNames finds the deployment and service account names from the objects.
func extractBaseNames(objects []runtime.Object, mapping *nameMapping) {
	for _, obj := range objects {
		switch typed := obj.(type) {
		case *appsv1.Deployment:
			mapping.deploymentName = typed.Name
		case *corev1.ServiceAccount:
			// Use the first service account as the base name
			if mapping.serviceAccountName == "" {
				mapping.serviceAccountName = typed.Name
			}
		}
	}
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

// buildResourceMappings creates mappings for all RBAC resources with OLM-generated names.
func buildResourceMappings(objects []runtime.Object, mapping *nameMapping) {
	baseName := getBaseName(mapping)

	roleCount := 0
	roleBindingCount := 0
	clusterRoleCount := 0
	clusterRoleBindingCount := 0

	for _, obj := range objects {
		switch typed := obj.(type) {
		case *rbacv1.Role:
			if isOLMGeneratedName(typed.Name) {
				newName := generateResourceName(baseName, "role", roleCount)
				mapping.oldToNew[resourceKey{name: typed.Name, kind: "Role"}] = newName
				roleCount++
			}

		case *rbacv1.RoleBinding:
			if isOLMGeneratedName(typed.Name) {
				newName := generateResourceName(baseName, "rolebinding", roleBindingCount)
				mapping.oldToNew[resourceKey{name: typed.Name, kind: "RoleBinding"}] = newName
				roleBindingCount++
			}

		case *rbacv1.ClusterRole:
			if isOLMGeneratedName(typed.Name) {
				newName := generateResourceName(baseName, "clusterrole", clusterRoleCount)
				mapping.oldToNew[resourceKey{name: typed.Name, kind: "ClusterRole"}] = newName
				clusterRoleCount++
			}

		case *rbacv1.ClusterRoleBinding:
			if isOLMGeneratedName(typed.Name) {
				newName := generateResourceName(baseName, "clusterrolebinding", clusterRoleBindingCount)
				mapping.oldToNew[resourceKey{name: typed.Name, kind: "ClusterRoleBinding"}] = newName
				clusterRoleBindingCount++
			}
		}
	}
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
func applyNameMapping(objects []runtime.Object, mapping *nameMapping) []runtime.Object {
	if len(mapping.oldToNew) == 0 {
		return objects
	}

	normalized := make([]runtime.Object, 0, len(objects))

	for _, obj := range objects {
		normalized = append(normalized, normalizeObject(obj, mapping))
	}

	return normalized
}

// normalizeObject normalizes a single object's name and cross-references.
func normalizeObject(obj runtime.Object, mapping *nameMapping) runtime.Object {
	// Try type assertions directly first (more reliable than GVK for typed objects)
	switch typed := obj.(type) {
	case *rbacv1.Role:
		return normalizeRole(typed, mapping)
	case *rbacv1.RoleBinding:
		return normalizeRoleBinding(typed, mapping)
	case *rbacv1.ClusterRole:
		return normalizeClusterRole(typed, mapping)
	case *rbacv1.ClusterRoleBinding:
		return normalizeClusterRoleBinding(typed, mapping)
	case *corev1.ServiceAccount:
		return normalizeServiceAccount(typed, mapping)
	case *appsv1.Deployment:
		return normalizeDeployment(typed, mapping)
	case *admissionregistrationv1.ValidatingWebhookConfiguration:
		return normalizeValidatingWebhook(typed, mapping)
	case *admissionregistrationv1.MutatingWebhookConfiguration:
		return normalizeMutatingWebhook(typed, mapping)
	default:
		return obj
	}
}

// normalizeServiceAccount normalizes a ServiceAccount's name.
func normalizeServiceAccount(sa *corev1.ServiceAccount, mapping *nameMapping) runtime.Object {
	normalized := sa.DeepCopy()
	key := resourceKey{name: normalized.Name, kind: "ServiceAccount"}
	if newName, ok := mapping.oldToNew[key]; ok {
		normalized.Name = newName
	}

	return normalized
}

// normalizeRole normalizes a Role's name.
func normalizeRole(role *rbacv1.Role, mapping *nameMapping) runtime.Object {
	normalized := role.DeepCopy()
	key := resourceKey{name: normalized.Name, kind: "Role"}
	if newName, ok := mapping.oldToNew[key]; ok {
		normalized.Name = newName
	}

	return normalized
}

// normalizeRoleBinding normalizes a RoleBinding's name and roleRef.
func normalizeRoleBinding(rb *rbacv1.RoleBinding, mapping *nameMapping) runtime.Object {
	normalized := rb.DeepCopy()

	// Update the RoleBinding's own name
	key := resourceKey{name: normalized.Name, kind: "RoleBinding"}
	if newName, ok := mapping.oldToNew[key]; ok {
		normalized.Name = newName
	}

	// Update the roleRef to point to the normalized Role name
	roleKey := resourceKey{name: normalized.RoleRef.Name, kind: "Role"}
	if newRoleName, ok := mapping.oldToNew[roleKey]; ok {
		normalized.RoleRef.Name = newRoleName
	}

	return normalized
}

// normalizeClusterRole normalizes a ClusterRole's name.
func normalizeClusterRole(cr *rbacv1.ClusterRole, mapping *nameMapping) runtime.Object {
	normalized := cr.DeepCopy()
	key := resourceKey{name: normalized.Name, kind: "ClusterRole"}
	if newName, ok := mapping.oldToNew[key]; ok {
		normalized.Name = newName
	}

	return normalized
}

// normalizeClusterRoleBinding normalizes a ClusterRoleBinding's name and roleRef.
func normalizeClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, mapping *nameMapping) runtime.Object {
	normalized := crb.DeepCopy()

	// Update the ClusterRoleBinding's own name
	key := resourceKey{name: normalized.Name, kind: "ClusterRoleBinding"}
	if newName, ok := mapping.oldToNew[key]; ok {
		normalized.Name = newName
	}

	// Update the roleRef to point to the normalized ClusterRole name
	roleKey := resourceKey{name: normalized.RoleRef.Name, kind: "ClusterRole"}
	if newClusterRoleName, ok := mapping.oldToNew[roleKey]; ok {
		normalized.RoleRef.Name = newClusterRoleName
	}

	return normalized
}

// normalizeDeployment normalizes serviceAccountName references in a Deployment.
func normalizeDeployment(dep *appsv1.Deployment, mapping *nameMapping) runtime.Object {
	normalized := dep.DeepCopy()

	// Update serviceAccountName if it was renamed
	saName := normalized.Spec.Template.Spec.ServiceAccountName
	if saName != "" {
		saKey := resourceKey{name: saName, kind: "ServiceAccount"}
		if newSAName, ok := mapping.oldToNew[saKey]; ok {
			normalized.Spec.Template.Spec.ServiceAccountName = newSAName
		}
	}

	return normalized
}

// normalizeValidatingWebhook normalizes webhook configuration names.
func normalizeValidatingWebhook(
	vwc *admissionregistrationv1.ValidatingWebhookConfiguration,
	mapping *nameMapping,
) runtime.Object {
	normalized := vwc.DeepCopy()
	normalized.Name = normalizeWebhookName(normalized.Name, mapping.deploymentName, "validating")

	return normalized
}

// normalizeMutatingWebhook normalizes webhook configuration names.
func normalizeMutatingWebhook(
	mwc *admissionregistrationv1.MutatingWebhookConfiguration,
	mapping *nameMapping,
) runtime.Object {
	normalized := mwc.DeepCopy()
	normalized.Name = normalizeWebhookName(normalized.Name, mapping.deploymentName, "mutating")

	return normalized
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
