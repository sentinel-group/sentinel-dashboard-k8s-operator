package controllers

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sentinelv1alpha1 "github.com/sentinel-group/sentinel-dashboard-k8s-operator/api/v1alpha1"
)

func MutateService(instance *sentinelv1alpha1.Dashboard, svc *corev1.Service) {
	svc.Labels = map[string]string{
		"app": instance.Name,
	}

	ports := []corev1.ServicePort{
		{
			Name:     "http",
			Port:     instance.Spec.Ports[0].Port,
			Protocol: "TCP",
		},
	}
	if strings.EqualFold(string(instance.Spec.Type), "NodePort") || strings.EqualFold(string(instance.Spec.Type), "LoadBalancer") {
		ports = []corev1.ServicePort{
			{
				Name:     "http",
				Port:     instance.Spec.Ports[0].Port,
				Protocol: "TCP",
				NodePort: instance.Spec.Ports[0].NodePort,
			},
		}
	}

	svc.Spec = corev1.ServiceSpec{
		Type: instance.Spec.Type,
		Selector: map[string]string{
			"app": instance.Name,
		},
		Ports: ports,
	}
}

func MutateDeployment(instance *sentinelv1alpha1.Dashboard, deploy *appsv1.Deployment) {
	labels := map[string]string{"app": instance.Name}
	deploy.Spec = appsv1.DeploymentSpec{
		Replicas: instance.Spec.Replicas,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers: newContainers(instance),
			},
		},
		Selector: &metav1.LabelSelector{MatchLabels: labels},
	}
}

func newContainers(sentinel *sentinelv1alpha1.Dashboard) []corev1.Container {
	env := []corev1.EnvVar{
		{
			Name:  "NACOS_ADDRESS",
			Value: sentinel.Name + "." + sentinel.Namespace + ":" + "8848",
		},
		{
			Name:  "NACOS_USERNAME",
			Value: "nacos",
		},
		{
			Name:  "NACOS_PASSWORD",
			Value: "nacos",
		},
	}
	return []corev1.Container{
		{
			Name:  sentinel.Name,
			Image: sentinel.Spec.Image,
			//Ports:           containerPorts,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Resources:       sentinel.Spec.Resources,
			Env:             env,
		},
	}
}
