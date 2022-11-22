package controllers

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sentinelv1alpha1 "github.com/sentinel-group/sentinel-dashboard-k8s-operator/api/v1alpha1"
)

func (r *DashboardReconciler) UpdateCondition(ctx context.Context, instance *sentinelv1alpha1.Dashboard,
	conditionType sentinelv1alpha1.DashboardConditionType, status metav1.ConditionStatus, reasons ...string) error {

	var reason, message string

	switch len(reasons) {
	case 0:
	case 1:
		reason = reasons[0]
	case 2:
		reason = reasons[1]
		message = reasons[2]
	default:
		return errors.Errorf("expecting reason and message, but got %d params", len(reasons))
	}

	now := metav1.Now()

	for i, cond := range instance.Status.Conditions {
		if cond.Type == string(conditionType) {
			if cond.LastTransitionTime.IsZero() || cond.Status != status {
				now.DeepCopyInto(&cond.LastTransitionTime)
			}

			cond.Status = status
			cond.Reason = reason
			cond.Message = message

			instance.Status.Conditions[i] = cond
			return nil
		}
	}

	cond := sentinelv1alpha1.DashboardCondition{
		Type:               string(conditionType),
		Status:             status,
		ObservedGeneration: 0,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	instance.Status.Conditions = append(instance.Status.Conditions, cond)

	return nil
}

func (r *DashboardReconciler) GetCondition(ctx context.Context, instance *sentinelv1alpha1.Dashboard,
	conditionType sentinelv1alpha1.DashboardConditionType) sentinelv1alpha1.DashboardCondition {

	for _, cond := range instance.Status.Conditions {
		if cond.Type == string(conditionType) {
			return cond
		}
	}

	return sentinelv1alpha1.DashboardCondition{
		Type:   string(conditionType),
		Status: metav1.ConditionUnknown,
	}
}

func (r *DashboardReconciler) UpdateStatus(ctx context.Context, instance *sentinelv1alpha1.Dashboard) error {
	logger := log.FromContext(ctx)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		errStatus := r.Status().Update(ctx, instance)
		logger.Info("updated dashboard status")
		return errStatus
	}); err != nil {
		logger.Error(err, "failed updated dashboard status")
		return err
	}
	return nil
}
