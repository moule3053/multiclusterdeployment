package controllers

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	mcdv1 "moule.com/multiclusterdeployment/api/v1"

	"k8s.io/client-go/tools/clientcmd"
)

const multiClusterDeploymentFinalizer = "finalizer.multiclusterdeployment.mcd.moule.com"

// MultiClusterDeploymentReconciler reconciles a MultiClusterDeployment object
type MultiClusterDeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=mcd.moule.com,resources=multiclusterdeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mcd.moule.com,resources=multiclusterdeployments/status,verbs=get;update;patch

func (r *MultiClusterDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("multiclusterdeployment", req.NamespacedName)

	// Fetch the MultiClusterDeployment instance
	mcd := &mcdv1.MultiClusterDeployment{}
	err := r.Get(ctx, req.NamespacedName, mcd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("MultiClusterDeployment resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get MultiClusterDeployment.")
		return ctrl.Result{}, err
	}

	// Check if the MultiClusterDeployment is being deleted
	if !mcd.ObjectMeta.DeletionTimestamp.IsZero() {
		// Handle deletion by removing the Deployment from all target clusters
		if containsString(mcd.ObjectMeta.Finalizers, multiClusterDeploymentFinalizer) {
			// Handle cleanup logic here
			err := r.cleanupDeployments(ctx, mcd)
			if err != nil {
				return ctrl.Result{}, err
			}

			// Remove finalizer
			mcd.ObjectMeta.Finalizers = removeString(mcd.ObjectMeta.Finalizers, multiClusterDeploymentFinalizer)
			if err := r.Update(ctx, mcd); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if it's not present
	if !containsString(mcd.ObjectMeta.Finalizers, multiClusterDeploymentFinalizer) {
		mcd.ObjectMeta.Finalizers = append(mcd.ObjectMeta.Finalizers, multiClusterDeploymentFinalizer)
		if err := r.Update(ctx, mcd); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Process each target cluster
	for _, cluster := range mcd.Spec.Clusters {
		// Get the target cluster's kubeconfig ConfigMap
		kubeconfigCM := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: cluster + "-kubeconfig", Namespace: mcd.Namespace}, kubeconfigCM)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "Failed to get kubeconfig for target cluster", "cluster", cluster)
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		// Get the kubeconfig data from the ConfigMap
		kubeconfigData, ok := kubeconfigCM.Data["kubeconfig"]
		if !ok {
			log.Error(errors.New("kubeconfig not found in ConfigMap"), "Failed to get kubeconfig for target cluster", "cluster", cluster)
			return ctrl.Result{}, nil
		}

		// Create a new client for the target cluster using the kubeconfig
		clusterClient, err := r.newClientForCluster([]byte(kubeconfigData))
		if err != nil {
			log.Error(err, "Failed to create client for target cluster", "cluster", cluster)
			continue
		}

		// Create/update/delete the Deployment on the target cluster as needed
		err = r.reconcileDeployment(ctx, clusterClient, mcd)
		if err != nil {
			log.Error(err, "Failed to reconcile Deployment on target cluster", "cluster", cluster)
			continue
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcdv1.MultiClusterDeployment{}).
		Complete(r)
}

// Implement helper functions getKubeconfigForCluster, newClientForCluster, and reconcileDeployment here

func (r *MultiClusterDeploymentReconciler) newClientForCluster(kubeconfig []byte) (client.Client, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	return client.New(cfg, client.Options{Scheme: r.Scheme})
}

func (r *MultiClusterDeploymentReconciler) reconcileDeployment(ctx context.Context, c client.Client, mcd *mcdv1.MultiClusterDeployment) error {
	labels := map[string]string{
		"app": mcd.Spec.Name,
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mcd.Spec.Name,
			Namespace: mcd.Namespace,
		},
	}

	replicas := mcd.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	_, err := controllerutil.CreateOrUpdate(ctx, c, deployment, func() error {
		deployment.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
		}
		deployment.Spec.Template.ObjectMeta.Labels = labels
		deployment.Spec.Replicas = &replicas
		deployment.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:  mcd.Spec.Name,
				Image: mcd.Spec.Image,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    mcd.Spec.CPURequest[corev1.ResourceCPU],
						corev1.ResourceMemory: mcd.Spec.MemoryRequest[corev1.ResourceMemory],
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    mcd.Spec.CPULimit[corev1.ResourceCPU],
						corev1.ResourceMemory: mcd.Spec.MemoryLimit[corev1.ResourceMemory],
					},
				},
			},
		}
		return nil
	})

	return err
}

func (r *MultiClusterDeploymentReconciler) cleanupDeployments(ctx context.Context, mcd *mcdv1.MultiClusterDeployment) error {
	log := r.Log.WithValues("multiclusterdeployment", mcd.Name)

	for _, cluster := range mcd.Spec.Clusters {
		// Get the target cluster's kubeconfig ConfigMap
		kubeconfigCM := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: cluster + "-kubeconfig", Namespace: mcd.Namespace}, kubeconfigCM)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "Failed to get kubeconfig for target cluster", "cluster", cluster)
				return nil
			}
			return err
		}

		// Get the kubeconfig data from the ConfigMap
		kubeconfigData, ok := kubeconfigCM.Data["kubeconfig"]
		if !ok {
			log.Error(errors.New("kubeconfig not found in ConfigMap"), "Failed to get kubeconfig for target cluster", "cluster", cluster)
			return nil
		}

		// Create a new client for the target cluster using the kubeconfig
		clusterClient, err := r.newClientForCluster([]byte(kubeconfigData))
		if err != nil {
			log.Error(err, "Failed to create client for target cluster", "cluster", cluster)
			continue
		}

		// Delete the Deployment on the target cluster
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mcd.Spec.Name,
				Namespace: mcd.Namespace,
			},
		}

		err = clusterClient.Delete(ctx, deployment)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to delete Deployment on target cluster", "cluster", cluster)
				return err
			}
		}
	}

	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	newSlice := []string{}
	for _, item := range slice {
		if item != s {
			newSlice = append(newSlice, item)
		}
	}
	return newSlice
}
