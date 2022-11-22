package controllers

import (
	"context"

	sentinelv1alpha1 "github.com/sentinel-group/sentinel-dashboard-k8s-operator/api/v1alpha1"
)

// GetHealth todo get dashboard health
func (c *DashboardReconciler) GetHealth(ctx context.Context, instance *sentinelv1alpha1.Dashboard) (bool, error) {
	return true, nil
}
