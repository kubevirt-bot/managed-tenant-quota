package namespaced

import (
	"fmt"
	utils2 "kubevirt.io/managed-tenant-quota/pkg/mtq-operator/resources/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

const (
	mtqLockResourceName = "mtq-lock"
)

func createMTQLockResources(args *FactoryArgs) []client.Object {
	return []client.Object{
		createMTQLockServiceAccount(),
		createMTQLockService(),
		createMTQLockDeployment(args.MTQLockImage, args.KVNamespace, args.PullPolicy, args.ImagePullSecrets, args.PriorityClassName, args.Verbosity, args.InfraNodePlacement),
	}
}

func createMTQLockServiceAccount() *corev1.ServiceAccount {
	return utils2.ResourceBuilder.CreateServiceAccount(mtqLockResourceName)
}

func createMTQLockService() *corev1.Service {
	service := utils2.ResourceBuilder.CreateService("mtq-lock", utils2.MTQLabel, mtqLockResourceName, nil)
	service.Spec.Type = corev1.ServiceTypeNodePort
	service.Spec.Ports = []corev1.ServicePort{
		{
			Port: 443,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: 8443,
			},
			Protocol: corev1.ProtocolTCP,
		},
	}
	return service
}

func createMTQLockDeployment(image, kvNs string, pullPolicy string, imagePullSecrets []corev1.LocalObjectReference, priorityClassName string, verbosity string, infraNodePlacement *sdkapi.NodePlacement) *appsv1.Deployment {
	defaultMode := corev1.ConfigMapVolumeSourceDefaultMode
	deployment := utils2.CreateDeployment(mtqLockResourceName, utils2.MTQLabel, mtqLockResourceName, mtqLockResourceName, imagePullSecrets, 1, infraNodePlacement)
	if priorityClassName != "" {
		deployment.Spec.Template.Spec.PriorityClassName = priorityClassName
	}
	container := utils2.CreateContainer(mtqLockResourceName, image, verbosity, pullPolicy)
	container.Ports = createMTQLockPorts()

	container.Env = []corev1.EnvVar{
		{
			Name: utils2.InstallerPartOfLabel,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  fmt.Sprintf("metadata.labels['%s']", utils2.AppKubernetesPartOfLabel),
				},
			},
		},
		{
			Name: utils2.InstallerVersionLabel,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  fmt.Sprintf("metadata.labels['%s']", utils2.AppKubernetesVersionLabel),
				},
			},
		},
		{
			Name:  utils2.TlsLabel,
			Value: "true",
		},
		{
			Name:  utils2.KubevirtInstallNamespace,
			Value: kvNs,
		},
	}
	container.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"cat", "/tmp/ready"},
			},
		},
		InitialDelaySeconds: 2,
		PeriodSeconds:       5,
		FailureThreshold:    3,
		SuccessThreshold:    1,
		TimeoutSeconds:      1,
	}
	container.Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("50Mi"),
		},
	}
	container.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "tls",
			MountPath: "/etc/admission-webhook/tls",
			ReadOnly:  true,
		},
	}
	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}

	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  "mtq-lock-server-cert",
					DefaultMode: &defaultMode,
				},
			},
		},
	}
	return deployment
}

func createMTQLockPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: 8443,
			Protocol:      "TCP",
		},
	}
}