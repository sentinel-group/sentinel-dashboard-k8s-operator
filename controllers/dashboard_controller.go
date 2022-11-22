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
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sentinelv1alpha1 "github.com/sentinel-group/sentinel-dashboard-k8s-operator/api/v1alpha1"
)

// DashboardReconciler reconciles a Dashboard object
type DashboardReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=sentinel.sentinelguard.io,resources=dashboards,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sentinel.sentinelguard.io,resources=dashboards/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sentinel.sentinelguard.io,resources=dashboards/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=service,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Dashboard object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *DashboardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("start reconcile")

	var instance sentinelv1alpha1.Dashboard
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("sentinel instance not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get sentinel instance")
		return ctrl.Result{}, err
	}
	logger.Info("sentinel instance: " + instance.String())

	var g errgroup.Group
	g.Go(func() error {
		err := r.UpdateAppliedStatus(ctx, &instance)
		return errors.Wrapf(err, "type=%s", sentinelv1alpha1.ReadyConditionType)
	})
	g.Go(func() error {
		err := r.UpdateReadyStatus(ctx, &instance)
		return errors.Wrapf(err, "type=%s", sentinelv1alpha1.ReadyConditionType)
	})
	err := g.Wait()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed set status")
	}

	err = r.UpdateStatus(ctx, &instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("success reconcile")
	return ctrl.Result{}, nil
}

func (r *DashboardReconciler) UpdateAppliedStatus(ctx context.Context, instance *sentinelv1alpha1.Dashboard) error {
	logger := log.FromContext(ctx)
	switch r.GetCondition(ctx, instance, sentinelv1alpha1.AppliedConditionType).Status {
	case metav1.ConditionTrue:
		return nil
	default:
		err := r.UpdateCondition(ctx, instance, sentinelv1alpha1.AppliedConditionType, metav1.ConditionFalse)
		if err != nil {
			return errors.Wrapf(err, "failed updating applied status, value=%s", corev1.ConditionFalse)
		}

		var deploy appsv1.Deployment
		deploy.Name = instance.Name
		deploy.Namespace = instance.Namespace
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			result, err := controllerutil.CreateOrUpdate(ctx, r.Client, &deploy, func() error {
				MutateDeployment(instance, &deploy)
				return controllerutil.SetControllerReference(instance, &deploy, r.Scheme)
			})
			logger.Info("updated deployment", "result", result, "deployment name", deploy.Name, "deployment namespace", deploy.Namespace)
			return err
		}); err != nil {
			condErr := r.UpdateCondition(ctx, instance, sentinelv1alpha1.AppliedConditionType, metav1.ConditionFalse, err.Error())
			if condErr != nil {
				return errors.Wrapf(err, "failed updating deployment")
			}
			return nil
		}

		var svc corev1.Service
		svc.Name = instance.Name
		svc.Namespace = instance.Namespace
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			result, err := ctrl.CreateOrUpdate(ctx, r.Client, &svc, func() error {
				MutateService(instance, &svc)
				return controllerutil.SetControllerReference(instance, &svc, r.Scheme)
			})
			logger.Info("updated service", "result", result, "service name", svc.Name, "deployment namespace", svc.Namespace)
			return err
		}); err != nil {
			condErr := r.UpdateCondition(ctx, instance, sentinelv1alpha1.AppliedConditionType, metav1.ConditionFalse, err.Error())
			if condErr != nil {
				return errors.Wrapf(err, "failed updating service")
			}
			return nil
		}

		err = r.UpdateCondition(ctx, instance, sentinelv1alpha1.AppliedConditionType, metav1.ConditionTrue)
		if err != nil {
			return errors.Wrapf(err, "failed updating conditions")
		}
	}

	return nil
}

func (r *DashboardReconciler) UpdateReadyStatus(ctx context.Context, instance *sentinelv1alpha1.Dashboard) error {
	logger := log.FromContext(ctx)
	health, err := r.GetHealth(ctx, instance)

	if err != nil {
		err = r.UpdateCondition(ctx, instance, sentinelv1alpha1.ReadyConditionType, metav1.ConditionFalse, errors.Cause(err).Error(), err.Error())
		if err != nil {
			return errors.Wrapf(err, "failed updating conditions")
		}
	} else {
		if health {
			err := r.UpdateCondition(ctx, instance, sentinelv1alpha1.ReadyConditionType, metav1.ConditionTrue)
			if err != nil {
				return errors.Wrapf(err, "failed updating conditions")
			}
		} else {
			logger.Info("not ready yet, trying again later")
			// todo delaySeconds to enqueue
			err := r.UpdateCondition(ctx, instance, sentinelv1alpha1.ReadyConditionType, metav1.ConditionFalse,
				"deployment or service", "deployment or service failed")
			if err != nil {
				return errors.Wrapf(err, "failed updating conditions")
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sentinelv1alpha1.Dashboard{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
