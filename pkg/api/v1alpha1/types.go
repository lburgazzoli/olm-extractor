// Package v1alpha1 contains API types for olm-extractor KRM function configuration.
//
// These types define the schema for functionConfig in ResourceList that users
// reference in their kustomization.yaml when using olm-extractor as a Kustomize generator.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExtractorSpec defines the configuration for extracting manifests from OLM bundles or catalogs.
type ExtractorSpec struct {
	// Source is either a bundle image or package name, depending on whether Catalog is set
	// - Bundle mode (no catalog): quay.io/example/operator-bundle:v1.0.0
	// - Catalog mode (with catalog): prometheus:0.56.0
	Source string `json:"source"`

	// Catalog enables catalog mode when present. When set, Source is interpreted as package[:version]
	// +optional
	Catalog *CatalogSource `json:"catalog,omitempty"`

	// Namespace is the target namespace for installation
	Namespace string `json:"namespace"`

	// Include contains jq expressions to include resources (repeatable, acts as OR)
	// +optional
	Include []string `json:"include,omitempty"`

	// Exclude contains jq expressions to exclude resources (repeatable, acts as OR, takes priority over include)
	// +optional
	Exclude []string `json:"exclude,omitempty"`

	// CertManager configures cert-manager integration for webhook certificates
	// +optional
	CertManager CertManagerConfig `json:"certManager,omitempty"`

	// Registry contains registry authentication and connection options
	// +optional
	Registry RegistryConfig `json:"registry,omitempty"`
}

// CatalogSource configures catalog-based bundle resolution.
type CatalogSource struct {
	// Source is the catalog container image reference (e.g., quay.io/catalog:latest)
	Source string `json:"source"`

	// Channel specifies the channel to use when resolving from catalog (defaults to package's defaultChannel)
	// +optional
	Channel string `json:"channel,omitempty"`
}

// CertManagerConfig configures cert-manager integration for webhook certificates.
type CertManagerConfig struct {
	// Enabled enables cert-manager integration for webhook certificates (default: true)
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// IssuerName is the name of the cert-manager Issuer or ClusterIssuer for webhook certificates.
	// If empty, auto-generates a self-signed Issuer named "<operator>-selfsigned"
	// +optional
	IssuerName string `json:"issuerName,omitempty"`

	// IssuerKind is the kind of cert-manager issuer: Issuer or ClusterIssuer.
	// If empty with empty issuer name, defaults to namespace-scoped Issuer
	// +optional
	IssuerKind string `json:"issuerKind,omitempty"`
}

// RegistryConfig contains registry authentication and connection options.
type RegistryConfig struct {
	// Insecure allows insecure connections to registries (HTTP or self-signed certificates)
	// +optional
	Insecure bool `json:"insecure,omitempty"`

	// Username for registry authentication (uses Docker config and credential helpers by default)
	// +optional
	Username string `json:"username,omitempty"`

	// Password for registry authentication (uses Docker config and credential helpers by default)
	// +optional
	Password string `json:"password,omitempty"`
}

// Extractor is the configuration for extracting manifests from OLM bundles or catalogs.
// This type is used as functionConfig in Kustomize ResourceList.
// +kubebuilder:object:root=true
type Extractor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ExtractorSpec `json:"spec"`
}
