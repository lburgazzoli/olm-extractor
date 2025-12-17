package v1alpha1

import (
	"github.com/lburgazzoli/olm-extractor/pkg/bundle"
	"github.com/lburgazzoli/olm-extractor/pkg/certmanager"
)

// Config holds all configuration for the application.
// This is the internal representation used by the extraction pipeline.
type Config struct {
	Namespace   string
	Include     []string
	Exclude     []string
	TempDir     string
	Catalog     string
	Channel     string
	CertManager certmanager.Config
	Registry    bundle.RegistryConfig
}

// ToConfig converts an Extractor to the internal Config structure and returns the source input.
// Returns (config, input, error) where:
// - config is the internal configuration.
// - input is either the bundle image or package[:version] depending on mode.
func (e *Extractor) ToConfig(tempDir string) (Config, string, error) {
	cfg := Config{
		Namespace: e.Spec.Namespace,
		Include:   e.Spec.Include,
		Exclude:   e.Spec.Exclude,
		TempDir:   tempDir,
		CertManager: certmanager.Config{
			Enabled:    boolValue(e.Spec.CertManager.Enabled, true),
			IssuerName: e.Spec.CertManager.IssuerName,
			IssuerKind: e.Spec.CertManager.IssuerKind,
		},
		Registry: bundle.RegistryConfig{
			Insecure: e.Spec.Registry.Insecure,
			Username: e.Spec.Registry.Username,
			Password: e.Spec.Registry.Password,
		},
	}

	var input string

	if e.Spec.Catalog != nil {
		// Catalog mode: source is package[:version]
		cfg.Catalog = e.Spec.Catalog.Source
		cfg.Channel = e.Spec.Catalog.Channel
		input = e.Spec.Source
	} else {
		// Bundle mode: source is bundle image
		input = e.Spec.Source
	}

	return cfg, input, nil
}

// boolValue returns the value of a bool pointer, or defaultVal if the pointer is nil.
func boolValue(ptr *bool, defaultVal bool) bool {
	if ptr == nil {
		return defaultVal
	}

	return *ptr
}
