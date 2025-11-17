// Package annotation provides functionality for parsing and validating
// Caronte secret sync annotations from Kubernetes pods.
package annotation

import (
	"fmt"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// AnnotationKey is the annotation key for secret sync configuration
	AnnotationKey = "jasm.codnod.io/secret-sync"
)

// PodAnnotation represents the parsed annotation structure.
type PodAnnotation struct {
	Provider   string            `yaml:"provider"`
	Path       string            `yaml:"path"`
	SecretName string            `yaml:"secretName"`
	Keys       map[string]string `yaml:"keys"`
}

// SecretSyncRequest represents a complete secret synchronization request.
type SecretSyncRequest struct {
	Provider   string
	SecretPath string
	SecretName string
	Namespace  string
	PodName    string
	PodUID     types.UID
	KeyMapping map[string]string
}

// ParseAnnotation parses the secret sync annotation from a pod.
// Returns SecretSyncRequest if valid, or an error if parsing fails.
func ParseAnnotation(annotationValue, namespace, podName string, podUID types.UID) (*SecretSyncRequest, error) {
	if annotationValue == "" {
		return nil, fmt.Errorf("annotation value is empty")
	}

	var podAnnotation PodAnnotation
	if err := yaml.Unmarshal([]byte(annotationValue), &podAnnotation); err != nil {
		return nil, fmt.Errorf("failed to parse annotation YAML: %w", err)
	}

	// Validate required fields
	if podAnnotation.Provider == "" {
		return nil, fmt.Errorf("provider field is required")
	}
	if podAnnotation.Path == "" {
		return nil, fmt.Errorf("path field is required")
	}
	if podAnnotation.SecretName == "" {
		return nil, fmt.Errorf("secretName field is required")
	}

	// TODO: Validate secretName is a valid Kubernetes name (DNS-1123 label)

	return &SecretSyncRequest{
		Provider:   podAnnotation.Provider,
		SecretPath: podAnnotation.Path,
		SecretName: podAnnotation.SecretName,
		Namespace:  namespace,
		PodName:    podName,
		PodUID:     podUID,
		KeyMapping: podAnnotation.Keys,
	}, nil
}
