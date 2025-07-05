package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/database/intermediaries"
	"github.com/Kaese72/adapter-attendant/internal/logging"
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

var ingestConfigName = "adapter-device-store-ingest"

type kubeHandle struct {
	clientSet *kubernetes.Clientset
	nameSpace string
}

func adapterLabels(adapterName string) map[string]string {
	return map[string]string{"huemie-adapter": adapterName, "huemie-purpose": "device-adapter"}
}

func (handle kubeHandle) GetAdapter(adapterName string, ctx context.Context) (intermediaries.Adapter, error) {
	deploy, err := handle.clientSet.AppsV1().Deployments(handle.nameSpace).Get(ctx, adapterName, metav1.GetOptions{})
	if err != nil {
		return intermediaries.Adapter{}, errors.Wrap(err, "failed to get deployments")
	}
	configMap, err := handle.clientSet.CoreV1().ConfigMaps(handle.nameSpace).Get(ctx, adapterName, metav1.GetOptions{})
	if err != nil {
		return intermediaries.Adapter{}, errors.Wrap(err, "failed to get config maps")
	}
	service, err := handle.clientSet.CoreV1().Services(handle.nameSpace).Get(ctx, adapterName, metav1.GetOptions{})
	if err != nil {
		return intermediaries.Adapter{}, errors.Wrap(err, "failed to get service")
	}
	//FIXME So far a deployment only consists of one container, but making that assumption is bad practice
	deployImage := deploy.Spec.Template.Spec.Containers[0].Image
	splitImage := strings.SplitN(deployImage, ":", 2)
	if len(splitImage) != 2 {
		return intermediaries.Adapter{}, errors.Errorf("could not reasonably split image into name and version, '%s'", deployImage)
	}
	return intermediaries.Adapter{
		Name: adapterName,
		Image: &intermediaries.Image{
			Name: splitImage[0],
			Tag:  splitImage[1],
		},
		// FIXME Here we will also need secrets and hide HUEMIE_ configuration
		Config: configMap.Data,
		// FIXME We need to come up with a way to make address more dynamic and support more protocols
		// Consider labels in kubernetes, or maybe its time to implement a database
		Address: fmt.Sprintf("http://%s:8080", service.Spec.ClusterIP),
	}, nil
}

func (handle kubeHandle) GetAdapters(ctx context.Context) ([]string, error) {
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

// bootstrapBackend intends to setup config maps and other resources
// for use with the adapter attendant
func (handle kubeHandle) bootstrapBackend() error {
	_, err := handle.clientSet.CoreV1().Namespaces().Get(context.TODO(), handle.nameSpace, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get namespaces")
	}

	bootConf := coreapplyv1.ConfigMapApplyConfiguration{
		TypeMetaApplyConfiguration:   *metaapplyv1.TypeMeta().WithAPIVersion("v1").WithKind("ConfigMap"),
		ObjectMetaApplyConfiguration: metaapplyv1.ObjectMeta().WithName(ingestConfigName),
		Data: map[string]string{
			// FIXME Make configurable
			"ENROLL_STORE": config.Loaded.DeviceStoreURL,
		},
	}
	_, err = handle.clientSet.CoreV1().ConfigMaps(handle.nameSpace).Apply(context.TODO(), &bootConf, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	if err != nil {
		return errors.Wrap(err, "failed to apply config map")
	}
	return nil
}

func (handle kubeHandle) applyConfig(name string, config map[string]string, ctx context.Context) (*corev1.ConfigMap, error) {
	// FIXME Define builtin non-overridable configuration
	config["ENROLL_ADAPTER_KEY"] = name
	adapterConfig := coreapplyv1.ConfigMapApplyConfiguration{
		TypeMetaApplyConfiguration:   *metaapplyv1.TypeMeta().WithKind("ConfigMap").WithAPIVersion("v1"),
		ObjectMetaApplyConfiguration: metaapplyv1.ObjectMeta().WithName(name).WithNamespace(handle.nameSpace),
		Data:                         config,
	}
	configMap, err := handle.clientSet.CoreV1().ConfigMaps(handle.nameSpace).Apply(ctx, &adapterConfig, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	return configMap, errors.Wrap(err, "failed to apply config map")
}

func (handle kubeHandle) applyDeployment(name string, image intermediaries.Image, ctx context.Context) (*appsv1.Deployment, *corev1.Service, error) {
	podLabels := adapterLabels(name)
	selector := metaapplyv1.LabelSelector().WithMatchLabels(podLabels)
	privateEnvs := coreapplyv1.EnvFromSource().WithConfigMapRef(coreapplyv1.ConfigMapEnvSource().WithName(name))
	storeEnvs := coreapplyv1.EnvFromSource().WithConfigMapRef(coreapplyv1.ConfigMapEnvSource().WithName(ingestConfigName))
	containerSpec := coreapplyv1.Container().WithName(name).WithImage(fmt.Sprintf("%s:%s", image.Name, image.Tag)).WithEnvFrom(privateEnvs, storeEnvs)
	podSpec := coreapplyv1.PodSpec().WithContainers(containerSpec)
	templateSpec := coreapplyv1.PodTemplateSpec().WithLabels(podLabels).WithSpec(podSpec)
	deploymentSpec := appsapplyv1.DeploymentSpec().WithReplicas(1).WithSelector(selector).WithTemplate(templateSpec)
	deployment := appsapplyv1.Deployment(name, handle.nameSpace).WithSpec(deploymentSpec).WithLabels(podLabels)

	appliedDeployment, err := handle.clientSet.AppsV1().Deployments(handle.nameSpace).Apply(context.TODO(), deployment, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to apply deployment")
	}
	servicePortSpec := coreapplyv1.ServicePort().WithAppProtocol("TCP").WithPort(8080).WithTargetPort(intstr.FromInt(8080))
	serviceSpec := coreapplyv1.ServiceSpec().WithSelector(podLabels).WithPorts(servicePortSpec)
	service := coreapplyv1.Service(name, handle.nameSpace).WithSpec(serviceSpec)

	appliedService, err := handle.clientSet.CoreV1().Services(handle.nameSpace).Apply(context.TODO(), service, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to apply config map")
	}
	return appliedDeployment, appliedService, nil
}

func (handle kubeHandle) ApplyAdapter(adapter intermediaries.Adapter, ctx context.Context) (intermediaries.Adapter, error) {
	// FIXME This function is a piece of crap. I need to figure out a way to make this more REST-y while still;
	// * Preventing configuration being created without a deployment
	// * Preventing deployment from being created without configuration
	// * Allow deployment be updated (image name/tag) without supplying configuration
	// * Allow configuration be updated without supplying image information
	// FIXME If the context is cancelled at the wrong time we may leave Kubernets in an inconsitent state
	if adapter.Config != nil {
		// If config is supplied we should apply a ConfigMap
		_, err := handle.applyConfig(adapter.Name, adapter.Config, ctx)
		if err != nil {
			logging.Error("Error applying config map", ctx, map[string]interface{}{"ERROR": err.Error()})
			return intermediaries.Adapter{}, errors.Wrap(err, "failed to apply config map")
		}
	} else {
		// If we do not post config, we need to make sure we at least have one already present
		_, err := handle.clientSet.CoreV1().ConfigMaps(handle.nameSpace).Get(context.TODO(), adapter.Name, metav1.GetOptions{})
		if err != nil {
			logging.Error("No config, and none present", ctx, map[string]interface{}{"ERROR": err.Error()})
			return intermediaries.Adapter{}, errors.Wrap(err, "failed to get config map when none was supplied by user")
		}
	}

	if adapter.Image != nil {
		// If image is set, we
		_, _, err := handle.applyDeployment(adapter.Name, *adapter.Image, ctx)
		if err != nil {
			return adapter, err
		}
	}
	return adapter, nil
}

func NewPureK8sBackend(conf config.Kubernetes) (AdapterAttendantDB, error) {
	// FIXME Do we want any other kind?
	var kubeConf *rest.Config = nil
	if conf.KubeConfigPath != "" {
		var err error = nil
		kubeConf, err = clientcmd.BuildConfigFromFlags("", conf.KubeConfigPath)
		if err != nil {
			return nil, err
		}
	} else if conf.InCluster {
		var err error = nil
		kubeConf, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("no valid Kubernetes config provided")
	}

	clientSet, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		return nil, err
	}
	handle := kubeHandle{
		clientSet: clientSet,
		nameSpace: conf.NameSpace,
	}
	return handle, handle.bootstrapBackend()
}
