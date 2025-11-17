// Package events provides helper functions for emitting Kubernetes events
// during secret synchronization operations.
package events

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	// EventReasonSecretSyncSuccess indicates successful secret synchronization
	EventReasonSecretSyncSuccess = "SecretSyncSuccess"

	// EventReasonSecretSyncFailed indicates failed secret synchronization
	EventReasonSecretSyncFailed = "SecretSyncFailed"

	// EventReasonAnnotationInvalid indicates invalid annotation format
	EventReasonAnnotationInvalid = "AnnotationInvalid"

	// EventReasonProviderUnsupported indicates unsupported provider
	EventReasonProviderUnsupported = "ProviderUnsupported"

	// EventReasonSecretFetchFailed indicates failure to fetch secret from provider
	EventReasonSecretFetchFailed = "SecretFetchFailed"
)

// EmitSecretSyncSuccess emits a Normal event when secret sync succeeds.
func EmitSecretSyncSuccess(recorder record.EventRecorder, pod *corev1.Pod, secretName, provider, path string) {
	recorder.Eventf(pod, corev1.EventTypeNormal, EventReasonSecretSyncSuccess,
		"Successfully synchronized secret '%s' from %s (path: %s)", secretName, provider, path)
}

// EmitAnnotationInvalid emits a Warning event when annotation is invalid.
func EmitAnnotationInvalid(recorder record.EventRecorder, pod *corev1.Pod, err error) {
	recorder.Eventf(pod, corev1.EventTypeWarning, EventReasonAnnotationInvalid,
		"Invalid secret sync annotation: %v", err)
}

// EmitSecretFetchFailed emits a Warning event when fetching secret fails.
func EmitSecretFetchFailed(recorder record.EventRecorder, pod *corev1.Pod, provider, path string, err error) {
	recorder.Eventf(pod, corev1.EventTypeWarning, EventReasonSecretFetchFailed,
		"Failed to fetch secret from %s (path: %s): %v", provider, path, err)
}

// EmitProviderNotFound emits a Warning event when provider is not found in registry.
func EmitProviderNotFound(recorder record.EventRecorder, pod *corev1.Pod, provider string) {
	recorder.Eventf(pod, corev1.EventTypeWarning, EventReasonProviderUnsupported,
		"Provider '%s' not found in registry", provider)
}
