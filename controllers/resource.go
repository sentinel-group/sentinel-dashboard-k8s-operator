package controllers

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	trafficflowv1alpha1 "skoala.daocloud.io/sentinel-operator/api/v1alpha1"
)

func MutateService(sentinel *trafficflowv1alpha1.Sentinel, svc *corev1.Service) {
	svc.Labels = map[string]string{
		"app": sentinel.Name,
		// 监控使用
		"skoala.io/type": "sentinel",
	}

	ports := []corev1.ServicePort{
		{
			Name:     "http",
			Port:     sentinel.Spec.Ports[0].Port,
			Protocol: "TCP",
		},
		{
			Name:     "jmx-metrics",
			Port:     12345,
			Protocol: "TCP",
		},
	}
	if strings.EqualFold(string(sentinel.Spec.Type), "NodePort") || strings.EqualFold(string(sentinel.Spec.Type), "LoadBalancer") {
		ports = []corev1.ServicePort{
			{
				Name:     "http",
				Port:     sentinel.Spec.Ports[0].Port,
				Protocol: "TCP",
				NodePort: sentinel.Spec.Ports[0].NodePort,
			},
			{
				Name:     "jmx-metrics",
				Port:     12345,
				Protocol: "TCP",
			},
		}
	}

	svc.Spec = corev1.ServiceSpec{
		Type: sentinel.Spec.Type,
		Selector: map[string]string{
			"app": sentinel.Name,
		},
		Ports: ports,
	}
}

func MutateDeployment(sentinel *trafficflowv1alpha1.Sentinel, deploy *appsv1.Deployment) {
	labels := map[string]string{"app": sentinel.Name}
	deploy.Spec = appsv1.DeploymentSpec{
		Replicas: sentinel.Spec.Replicas,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
			},
			Spec: corev1.PodSpec{
				Containers: newContainers(sentinel),
			},
		},
		Selector: &metav1.LabelSelector{MatchLabels: labels},
	}
}

func newContainers(sentinel *trafficflowv1alpha1.Sentinel) []corev1.Container {
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
