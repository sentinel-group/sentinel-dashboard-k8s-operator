package controllers

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"

	sentinelv1alpha1 "github.com/sentinel-group/sentinel-dashboard-k8s-operator/api/v1alpha1"
)

func (r *DashboardReconciler) GetHealth(ctx context.Context, instance *sentinelv1alpha1.Dashboard) error {
	config := rest.CopyConfig(r.RestConfig)
	config.APIPath = "api"
	config.NegotiatedSerializer = serializer.NewCodecFactory(r.Scheme)
	config.GroupVersion = &corev1.SchemeGroupVersion
	client, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return errors.Wrap(err, "cannot get rest client")
	}

	if _, err := client.Get().
		Resource("services").
		Namespace(instance.GetNamespace()).
		Name(instance.Name + ":8080").
		SubResource("proxy").
		Suffix("/version").
		DoRaw(ctx); err != nil {
		return errors.Wrap(err, "cannot get health response")
	}

	return nil
}
