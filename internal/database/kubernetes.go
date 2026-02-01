package database

import (
	"context"
	"fmt"
	"time"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/logging"
	"github.com/Kaese72/adapter-attendant/internal/utility"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsapplyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	coreapplyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	metaapplyv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeHandle struct {
	clientSet *kubernetes.Clientset
	nameSpace string
}

func adapterLabels(adapterName string) map[string]string {
	return map[string]string{"huemie-adapter": adapterName, "huemie-purpose": "device-adapter"}
}

func (handle KubeHandle) GetAdapters(ctx context.Context) ([]string, error) {
	// FIXME Based LabelSelector on adapterLabels
	deployList, err := handle.clientSet.AppsV1().Deployments(handle.nameSpace).List(ctx, metav1.ListOptions{LabelSelector: "huemie-purpose=device-adapter"})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get deployments")
	}
	resList := []string{}
	for _, deploy := range deployList.Items {
		resList = append(resList, deploy.Name)
	}

	return resList, nil
}

func (handle KubeHandle) applyConfig(ctx context.Context, resourceName string, configuration map[string]string) (*corev1.ConfigMap, error) {
	// FIXME Define builtin non-overridable configuration
	// FIXME Deep copy
	adapterConfig := coreapplyv1.ConfigMapApplyConfiguration{
		TypeMetaApplyConfiguration:   *metaapplyv1.TypeMeta().WithKind("ConfigMap").WithAPIVersion("v1"),
		ObjectMetaApplyConfiguration: metaapplyv1.ObjectMeta().WithName(resourceName).WithNamespace(handle.nameSpace),
		Data:                         configuration,
	}
	configMap, err := handle.clientSet.CoreV1().ConfigMaps(handle.nameSpace).Apply(ctx, &adapterConfig, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	return configMap, errors.Wrap(err, "failed to apply config map")
}

func (handle KubeHandle) applyDeployment(resourceName string, image string, ctx context.Context) (*appsv1.Deployment, *corev1.Service, error) {
	// FIXME we assume names of sub-resources based on adapter name
	podLabels := adapterLabels(resourceName)
	selector := metaapplyv1.LabelSelector().WithMatchLabels(podLabels)
	privateEnvs := coreapplyv1.EnvFromSource().WithConfigMapRef(coreapplyv1.ConfigMapEnvSource().WithName(resourceName))
	containerSpec := coreapplyv1.Container().WithName(resourceName).WithImage(image).WithEnvFrom(privateEnvs)
	podSpec := coreapplyv1.PodSpec().WithContainers(containerSpec)
	templateSpec := coreapplyv1.PodTemplateSpec().WithLabels(podLabels).WithSpec(podSpec)
	deploymentSpec := appsapplyv1.DeploymentSpec().WithReplicas(1).WithSelector(selector).WithTemplate(templateSpec)
	deployment := appsapplyv1.Deployment(resourceName, handle.nameSpace).WithSpec(deploymentSpec).WithLabels(podLabels)
	appliedDeployment, err := handle.clientSet.AppsV1().Deployments(handle.nameSpace).Apply(ctx, deployment, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to apply deployment")
	}
	servicePortSpec := coreapplyv1.ServicePort().WithAppProtocol("TCP").WithPort(8080).WithTargetPort(intstr.FromInt(8080))
	serviceSpec := coreapplyv1.ServiceSpec().WithSelector(podLabels).WithPorts(servicePortSpec)
	service := coreapplyv1.Service(resourceName, handle.nameSpace).WithSpec(serviceSpec)

	appliedService, err := handle.clientSet.CoreV1().Services(handle.nameSpace).Apply(ctx, service, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to apply config map")
	}
	return appliedDeployment, appliedService, nil
}

func (handle KubeHandle) ApplyAdapter(ctx context.Context, adapterId int, image string, configuration map[string]string) error {
	// FIXME This function is a piece of crap. I need to figure out a way to make this more REST-y while still;
	// * Preventing configuration being created without a deployment
	// * Preventing deployment from being created without configuration
	// * Allow deployment be updated (image name/tag) without supplying configuration
	// * Allow configuration be updated without supplying image information
	// FIXME If the context is cancelled at the wrong time we may leave Kubernets in an inconsitent state
	// FIXME Replace with ArgoCD or similar?
	resourceName := fmt.Sprintf("adapter-%d", adapterId)
	jwtToken, err := utility.GenerateAdapterJWT(config.Loaded.Adapters.DeviceStoreJWTSecret, 24*30*12*time.Hour, adapterId)
	if err != nil {
		logging.Error("Error generating enrollment token", ctx, map[string]interface{}{"ERROR": err.Error()})
		return errors.Wrap(err, "failed to generate enrollment token")
	}
	// Add mandatory configuration that is not visible to user
	configuration["ENROLL_STORE"] = config.Loaded.Adapters.DeviceStoreURL
	configuration["ENROLL_TOKEN"] = jwtToken
	// If config is supplied we should apply a ConfigMap
	_, err = handle.applyConfig(ctx, resourceName, configuration)
	if err != nil {
		logging.Error("Error applying config map", ctx, map[string]interface{}{"ERROR": err.Error()})
		return errors.Wrap(err, "failed to apply config map")
	}
	// If image is set, we
	_, _, err = handle.applyDeployment(resourceName, image, ctx)
	if err != nil {
		logging.Error("Error applying deployment", ctx, map[string]interface{}{"ERROR": err.Error()})
		return errors.Wrap(err, "failed to apply deployment")
	}
	return nil
}

func NewPureK8sBackend(conf config.Kubernetes) (KubeHandle, error) {
	// FIXME Do we want any other kind?
	var kubeConf *rest.Config = nil
	if conf.KubeConfigPath != "" {
		var err error = nil
		kubeConf, err = clientcmd.BuildConfigFromFlags("", conf.KubeConfigPath)
		if err != nil {
			return KubeHandle{}, err
		}
	} else if conf.InCluster {
		var err error = nil
		kubeConf, err = rest.InClusterConfig()
		if err != nil {
			return KubeHandle{}, err
		}
	} else {
		return KubeHandle{}, fmt.Errorf("no valid Kubernetes config provided")
	}

	clientSet, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		return KubeHandle{}, err
	}
	handle := KubeHandle{
		clientSet: clientSet,
		nameSpace: conf.NameSpace,
	}
	return handle, nil
}
