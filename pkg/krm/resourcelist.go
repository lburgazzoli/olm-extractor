// Package krm provides support for Kubernetes Resource Model (KRM) function interface.
//
// This package implements the ResourceList I/O format used by Kustomize and other
// KRM-compatible tools. It handles reading function configuration from stdin and
// writing generated resources to stdout in the ResourceList format.
package krm

import (
	"errors"
	"fmt"
	"io"

	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/olm-extractor/pkg/api/v1alpha1"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
)

const (
	resourceListAPIVersion = "config.kubernetes.io/v1"
	resourceListKind       = "ResourceList"
	extractorKind          = "Extractor"
)

const (
	// SeverityError indicates a validation or execution error.
	SeverityError = "error"
	// SeverityWarning indicates a warning that should be reviewed.
	SeverityWarning = "warning"
	// SeverityInfo provides informational messages.
	SeverityInfo = "info"
)

// ResourceList represents the Kustomize KRM function ResourceList format.
// See: https://github.com/kubernetes-sigs/kustomize/blob/master/cmd/config/docs/api-conventions/functions-spec.md
type ResourceList struct {
	// APIVersion is always "config.kubernetes.io/v1"
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`

	// Kind is always "ResourceList"
	Kind string `json:"kind" yaml:"kind"`

	// Items contains the list of Kubernetes resources
	Items []*unstructured.Unstructured `json:"items" yaml:"items"`

	// FunctionConfig contains the configuration for the function
	FunctionConfig *unstructured.Unstructured `json:"functionConfig,omitempty" yaml:"functionConfig,omitempty"`

	// Results can be used to communicate validation errors or information back to the caller
	Results []Result `json:"results,omitempty" yaml:"results,omitempty"`
}

// Result represents a validation or informational message.
type Result struct {
	// Message is the human-readable message
	Message string `json:"message" yaml:"message"`

	// Severity indicates the severity level: "error", "warning", or "info"
	Severity string `json:"severity" yaml:"severity"`

	// ResourceRef identifies the resource this result applies to (optional)
	ResourceRef *ResourceRef `json:"resourceRef,omitempty" yaml:"resourceRef,omitempty"`

	// File identifies the file this result applies to (optional)
	File *File `json:"file,omitempty" yaml:"file,omitempty"`
}

// ResourceRef identifies a specific Kubernetes resource.
type ResourceRef struct {
	APIVersion string `json:"apiVersion"          yaml:"apiVersion"`
	Kind       string `json:"kind"                yaml:"kind"`
	Name       string `json:"name"                yaml:"name"`
	Namespace  string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// File identifies a file location.
type File struct {
	Path string `json:"path" yaml:"path"`
}

// ReadResourceList reads a ResourceList from the provided reader (typically stdin).
func ReadResourceList(r io.Reader) (*ResourceList, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	var rl ResourceList
	if err := yaml.Unmarshal(data, &rl); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ResourceList: %w", err)
	}

	if rl.APIVersion != resourceListAPIVersion {
		return nil, fmt.Errorf("unexpected APIVersion: got %q, want %q", rl.APIVersion, resourceListAPIVersion)
	}

	if rl.Kind != resourceListKind {
		return nil, fmt.Errorf("unexpected Kind: got %q, want %q", rl.Kind, resourceListKind)
	}

	return &rl, nil
}

// WriteResourceList writes a ResourceList to the provided writer (typically stdout).
func WriteResourceList(w io.Writer, rl *ResourceList) error {
	data, err := yaml.Marshal(rl)
	if err != nil {
		return fmt.Errorf("failed to marshal ResourceList: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write ResourceList: %w", err)
	}

	return nil
}

// FunctionConfig represents the parsed function configuration.
type FunctionConfig struct {
	// Kind is always "Extractor"
	Kind string

	// ExtractorConfig contains the extractor configuration
	ExtractorConfig *v1alpha1.Extractor
}

// ExtractFunctionConfig extracts and decodes the functionConfig from a ResourceList.
func ExtractFunctionConfig(rl *ResourceList) (*FunctionConfig, error) {
	if rl.FunctionConfig == nil {
		return nil, errors.New("functionConfig is required but not provided")
	}

	kind := rl.FunctionConfig.GetKind()
	if kind == "" {
		return nil, errors.New("functionConfig.kind is required")
	}

	if kind != extractorKind {
		return nil, fmt.Errorf("unsupported functionConfig kind: %q (expected %q)", kind, extractorKind)
	}

	var extractor v1alpha1.Extractor
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		rl.FunctionConfig.Object,
		&extractor,
	); err != nil {
		return nil, fmt.Errorf("failed to decode Extractor: %w", err)
	}

	return &FunctionConfig{
		Kind:            extractorKind,
		ExtractorConfig: &extractor,
	}, nil
}

// NewResourceList creates a new ResourceList with the standard API version and kind.
func NewResourceList() *ResourceList {
	return &ResourceList{
		APIVersion: resourceListAPIVersion,
		Kind:       resourceListKind,
		Items:      make([]*unstructured.Unstructured, 0),
	}
}

// AddErrorf adds an error result to the ResourceList.
func (rl *ResourceList) AddErrorf(format string, args ...any) {
	rl.Results = append(rl.Results, Result{
		Message:  fmt.Sprintf(format, args...),
		Severity: SeverityError,
	})
}

// AddWarningf adds a warning result to the ResourceList.
func (rl *ResourceList) AddWarningf(format string, args ...any) {
	rl.Results = append(rl.Results, Result{
		Message:  fmt.Sprintf(format, args...),
		Severity: SeverityWarning,
	})
}

// AddInfof adds an info result to the ResourceList.
func (rl *ResourceList) AddInfof(format string, args ...any) {
	rl.Results = append(rl.Results, Result{
		Message:  fmt.Sprintf(format, args...),
		Severity: SeverityInfo,
	})
}

// ToResourceList converts a slice of unstructured objects into a KRM ResourceList.
func ToResourceList(objects []*unstructured.Unstructured) *ResourceList {
	rl := NewResourceList()
	for _, obj := range objects {
		// Clean the object before adding to ResourceList
		cleanedObj := kube.CleanUnstructured(obj)
		rl.Items = append(rl.Items, cleanedObj)
	}

	return rl
}
