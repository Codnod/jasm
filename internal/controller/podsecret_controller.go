// Package controller implements the Kubernetes controller logic for watching
// pods and synchronizing secrets from external providers.
package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/codnod/jasm/internal/annotation"
	"github.com/codnod/jasm/internal/events"
	"github.com/codnod/jasm/internal/provider"
)

// PodSecretReconciler reconciles Pod objects with secret sync annotations.
type PodSecretReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	ProviderRegistry *provider.ProviderRegistry
}

const (
	// AnnotationKey is the annotation key for secret sync configuration.
	AnnotationKey = "jasm.codnod.io/secret-sync"
	// ManagedByLabel identifies secrets managed by JASM.
	ManagedByLabel = "app.kubernetes.io/managed-by"
	// ManagedByValue is the value for the managed-by label.
	ManagedByValue = "jasm"
	// SourcePathAnnotation tracks the external source path.
	SourcePathAnnotation = "jasm.codnod.io/source-path"
	// SyncedAtAnnotation tracks the last sync timestamp.
	SyncedAtAnnotation = "jasm.codnod.io/synced-at"
)

// Reconcile handles pod events and synchronizes secrets.
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
func (r *PodSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		log.Error(err, "unable to fetch Pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling pod", "namespace", pod.Namespace, "name", pod.Name)

	annotationValue, found := pod.Annotations[AnnotationKey]
	if !found {
		log.V(1).Info("Pod has no secret sync annotation, skipping")
		return ctrl.Result{}, nil
	}

	syncRequest, err := annotation.ParseAnnotation(annotationValue, pod.Namespace, pod.Name, pod.UID)
	if err != nil {
		log.Error(err, "Failed to parse annotation", "annotation", annotationValue)
		events.EmitAnnotationInvalid(r.Recorder, &pod, err)
		return ctrl.Result{}, nil
	}

	if syncRequest.Namespace != pod.Namespace {
		err := fmt.Errorf("annotation namespace mismatch: annotation specifies %s but pod is in %s", syncRequest.Namespace, pod.Namespace)
		log.Error(err, "Namespace validation failed")
		events.EmitAnnotationInvalid(r.Recorder, &pod, err)
		return ctrl.Result{}, nil
	}

	secretProvider := r.ProviderRegistry.Get(syncRequest.Provider)
	if secretProvider == nil {
		err := fmt.Errorf("unsupported provider: %s", syncRequest.Provider)
		log.Error(err, "Provider not found", "provider", syncRequest.Provider)
		events.EmitProviderNotFound(r.Recorder, &pod, syncRequest.Provider)
		return ctrl.Result{}, nil
	}

	log.Info("Fetching secret from provider", "provider", syncRequest.Provider, "path", syncRequest.SecretPath)
	secretData, err := secretProvider.FetchSecret(ctx, syncRequest.SecretPath)
	if err != nil {
		log.Error(err, "Failed to fetch secret", "provider", syncRequest.Provider, "path", syncRequest.SecretPath)
		events.EmitSecretFetchFailed(r.Recorder, &pod, syncRequest.Provider, syncRequest.SecretPath, err)
		return ctrl.Result{Requeue: true}, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      syncRequest.SecretName,
			Namespace: syncRequest.Namespace,
		},
	}

	err = r.Get(ctx, client.ObjectKeyFromObject(secret), secret)
	secretExists := !apierrors.IsNotFound(err)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "Failed to check if secret exists", "secret", syncRequest.SecretName)
		return ctrl.Result{}, err
	}

	secretStringData := make(map[string]string)

	// Apply key mappings if provided
	if len(syncRequest.KeyMapping) > 0 {
		for kubernetesKey, awsSecretKey := range syncRequest.KeyMapping {
			if value, exists := secretData[awsSecretKey]; exists {
				secretStringData[kubernetesKey] = value
				log.V(1).Info("Mapped secret key", "awsKey", awsSecretKey, "kubernetesKey", kubernetesKey)
			} else {
				log.Info("AWS secret key not found in fetched secret", "awsKey", awsSecretKey)
			}
		}
	} else {
		// If no key mappings, copy all keys as-is
		for k, v := range secretData {
			secretStringData[k] = v
		}
	}

	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels[ManagedByLabel] = ManagedByValue

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[SourcePathAnnotation] = syncRequest.SecretPath
	secret.Annotations[SyncedAtAnnotation] = time.Now().UTC().Format(time.RFC3339)

	secret.StringData = secretStringData
	secret.Type = corev1.SecretTypeOpaque

	if secretExists {
		log.Info("Updating existing secret", "secret", syncRequest.SecretName, "namespace", syncRequest.Namespace)
		if err := r.Update(ctx, secret); err != nil {
			log.Error(err, "Failed to update secret", "secret", syncRequest.SecretName)
			return ctrl.Result{}, err
		}
		log.Info("Secret updated successfully", "secret", syncRequest.SecretName)
	} else {
		log.Info("Creating new secret", "secret", syncRequest.SecretName, "namespace", syncRequest.Namespace)
		if err := r.Create(ctx, secret); err != nil {
			log.Error(err, "Failed to create secret", "secret", syncRequest.SecretName)
			return ctrl.Result{}, err
		}
		log.Info("Secret created successfully", "secret", syncRequest.SecretName)
	}

	events.EmitSecretSyncSuccess(r.Recorder, &pod, syncRequest.SecretName, syncRequest.Provider, syncRequest.SecretPath)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findPodsForSecret),
		).
		Complete(r)
}

// findPodsForSecret finds all pods that reference a deleted secret.
// This ensures that when a Caronte-managed secret is deleted, the pods
// that need it are reconciled and the secret is recreated.
func (r *PodSecretReconciler) findPodsForSecret(ctx context.Context, secret client.Object) []reconcile.Request {
	if secret.GetLabels()[ManagedByLabel] != ManagedByValue {
		return nil
	}

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(secret.GetNamespace())); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, pod := range podList.Items {
		if _, hasAnnotation := pod.Annotations[AnnotationKey]; !hasAnnotation {
			continue
		}

		syncRequest, err := annotation.ParseAnnotation(
			pod.Annotations[AnnotationKey],
			pod.Namespace,
			pod.Name,
			pod.UID,
		)
		if err != nil {
			continue
		}

		if syncRequest.SecretName == secret.GetName() {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(&pod),
			})
		}
	}

	return requests
}
