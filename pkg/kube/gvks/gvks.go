package gvks

import "k8s.io/apimachinery/pkg/runtime/schema"

// Core v1 resources.
var (
	Service = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}

	ServiceAccount = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ServiceAccount",
	}

	ConfigMap = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	Namespace = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	}

	PersistentVolume = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "PersistentVolume",
	}

	Node = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Node",
	}
)

// Apps v1 resources.
var (
	Deployment = schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}
)

// Admission registration resources.
var (
	ValidatingWebhookConfiguration = schema.GroupVersionKind{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "ValidatingWebhookConfiguration",
	}

	MutatingWebhookConfiguration = schema.GroupVersionKind{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "MutatingWebhookConfiguration",
	}
)

// ApiExtensions resources.
var (
	CustomResourceDefinition = schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	}

	CustomResourceDefinitionV1Beta1 = schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1beta1",
		Kind:    "CustomResourceDefinition",
	}
)

// RBAC resources.
var (
	Role = schema.GroupVersionKind{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1",
		Kind:    "Role",
	}

	RoleBinding = schema.GroupVersionKind{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1",
		Kind:    "RoleBinding",
	}

	ClusterRole = schema.GroupVersionKind{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1",
		Kind:    "ClusterRole",
	}

	ClusterRoleBinding = schema.GroupVersionKind{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1",
		Kind:    "ClusterRoleBinding",
	}
)

// Storage resources.
var (
	StorageClass = schema.GroupVersionKind{
		Group:   "storage.k8s.io",
		Version: "v1",
		Kind:    "StorageClass",
	}

	VolumeAttachment = schema.GroupVersionKind{
		Group:   "storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeAttachment",
	}

	CSIDriver = schema.GroupVersionKind{
		Group:   "storage.k8s.io",
		Version: "v1",
		Kind:    "CSIDriver",
	}

	CSINode = schema.GroupVersionKind{
		Group:   "storage.k8s.io",
		Version: "v1",
		Kind:    "CSINode",
	}

	VolumeSnapshotClass = schema.GroupVersionKind{
		Group:   "snapshot.storage.k8s.io",
		Version: "v1",
		Kind:    "VolumeSnapshotClass",
	}
)

// Scheduling resources.
var (
	PriorityClass = schema.GroupVersionKind{
		Group:   "scheduling.k8s.io",
		Version: "v1",
		Kind:    "PriorityClass",
	}
)

// Networking resources.
var (
	IngressClass = schema.GroupVersionKind{
		Group:   "networking.k8s.io",
		Version: "v1",
		Kind:    "IngressClass",
	}
)

// Node resources.
var (
	RuntimeClass = schema.GroupVersionKind{
		Group:   "node.k8s.io",
		Version: "v1",
		Kind:    "RuntimeClass",
	}
)

// Policy resources.
var (
	PodSecurityPolicy = schema.GroupVersionKind{
		Group:   "policy",
		Version: "v1beta1",
		Kind:    "PodSecurityPolicy",
	}
)

// Cert-manager resources.
var (
	Certificate = schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Certificate",
	}

	ClusterIssuer = schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "ClusterIssuer",
	}
)

// OLM resources.
var (
	ClusterServiceVersion = schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "ClusterServiceVersion",
	}
)

// ClusterScoped contains all cluster-scoped resource GVKs.
var ClusterScoped = map[schema.GroupVersionKind]bool{
	// Core v1
	Namespace:        true,
	PersistentVolume: true,
	Node:             true,
	// ApiExtensions
	CustomResourceDefinition: true,
	// RBAC
	ClusterRole:        true,
	ClusterRoleBinding: true,
	// Storage
	StorageClass:        true,
	VolumeAttachment:    true,
	CSIDriver:           true,
	CSINode:             true,
	VolumeSnapshotClass: true,
	// Scheduling
	PriorityClass: true,
	// Networking
	IngressClass: true,
	// Node
	RuntimeClass: true,
	// Policy
	PodSecurityPolicy: true,
	// Cert-manager
	ClusterIssuer: true,
	// Admission registration
	ValidatingWebhookConfiguration: true,
	MutatingWebhookConfiguration:   true,
}
