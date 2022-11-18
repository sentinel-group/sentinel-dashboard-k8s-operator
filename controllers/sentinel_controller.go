/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	trafficflowv1alpha1 "sentinelguard.io/sentinel-operator/api/v1alpha1"
)

// SentinelReconciler reconciles a Sentinel object
type SentinelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=sentinelguard.io,resources=sentinels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sentinelguard.io,resources=sentinels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sentinelguard.io,resources=sentinels/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=service,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Sentinel object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *SentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("start reconcile")

	var sentinel trafficflowv1alpha1.Sentinel
	if err := r.Get(ctx, req.NamespacedName, &sentinel); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("sentinel instance not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get sentinel instance")
		return ctrl.Result{}, err
	}
	logger.Info("sentinel instance: " + sentinel.String())

	var deploy appsv1.Deployment
	deploy.Name = sentinel.Name
	deploy.Namespace = sentinel.Namespace
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deploy, func() error {
			MutateDeployment(&sentinel, &deploy)
			return controllerutil.SetControllerReference(&sentinel, &deploy, r.Scheme)
		})
		logger.Info("updated deployment", "result", result, "deployment name", deploy.Name, "deployment namespace", deploy.Namespace)
		return err
	}); err != nil {
		logger.Error(err, "failed updated deployment", "deployment name", deploy.Name, "deployment namespace", deploy.Namespace)
		return ctrl.Result{}, err
	}

	var svc corev1.Service
	svc.Name = sentinel.Name
	svc.Namespace = sentinel.Namespace
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		result, err := ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
			MutateService(&sentinel, &svc)
			return controllerutil.SetControllerReference(&sentinel, &svc, r.Scheme)
		})
		logger.Info("updated service", "result", result, "service name", svc.Name, "deployment namespace", svc.Namespace)
		return err
	}); err != nil {
		logger.Error(err, "failed updated service", "service name", svc.Name, "service namespace", svc.Namespace)
		return ctrl.Result{}, err
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, req.NamespacedName, &sentinel); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("sentinel instance not found, ignoring since object must be deleted")
				return nil
			}
			logger.Error(err, "failed to get sentinel instance")
			return err
		}

		sentinel.Status.Phase = trafficflowv1alpha1.PhaseWaiting
		if len(deploy.Status.Conditions) > 0 {
			if deploy.Status.Conditions[0].Status == corev1.ConditionTrue && deploy.Status.ReadyReplicas == *sentinel.Spec.Replicas {
				sentinel.Status.Phase = trafficflowv1alpha1.PhaseRunning
			} else if deploy.Status.Conditions[0].Status == corev1.ConditionFalse {
				sentinel.Status.Phase = trafficflowv1alpha1.PhaseNotReady
			}
		}
		errStatus := r.Status().Update(ctx, &sentinel)
		logger.Info("updated sentinel status", "service name", svc.Name, "deployment namespace", svc.Namespace)
		return errStatus
	}); err != nil {
		logger.Error(err, "failed updated sentinel status", "sentinel name", sentinel.Name, "sentinel namespace", sentinel.Namespace)
		return ctrl.Result{}, err
	}

	logger.Info("success reconcile")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&trafficflowv1alpha1.Sentinel{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
