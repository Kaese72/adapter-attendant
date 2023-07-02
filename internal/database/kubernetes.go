package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/database/intermediaries"
	"github.com/Kaese72/huemie-lib/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsapplyv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	coreapplyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	metaapplyv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/kubernetes"
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

func (handle kubeHandle) GetAdapter(adapterName string) (intermediaries.Adapter, error) {
	deploy, err := handle.clientSet.AppsV1().Deployments(handle.nameSpace).Get(context.TODO(), adapterName, metav1.GetOptions{})
	if err != nil {
		return intermediaries.Adapter{}, err
	}
	configMap, err := handle.clientSet.CoreV1().ConfigMaps(handle.nameSpace).Get(context.TODO(), adapterName, metav1.GetOptions{})
	if err != nil {
		return intermediaries.Adapter{}, err
	}
	service, err := handle.clientSet.CoreV1().Services(handle.nameSpace).Get(context.TODO(), adapterName, metav1.GetOptions{})
	if err != nil {
		return intermediaries.Adapter{}, err
	}
	//FIXME So far a deployment only consists of one container, but making that assumption is bad practice
	deployImage := deploy.Spec.Template.Spec.Containers[0].Image
	splitImage := strings.SplitN(deployImage, ":", 2)
	if len(splitImage) != 2 {
		return intermediaries.Adapter{}, fmt.Errorf("image '%s' could not be reasonably split between image name and version", deployImage)
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

func (handle kubeHandle) GetAdapters() ([]string, error) {
	// FIXME Based LabelSelector on adapterLabels
	deployList, err := handle.clientSet.AppsV1().Deployments(handle.nameSpace).List(context.TODO(), metav1.ListOptions{LabelSelector: "huemie-purpose=device-adapter"})
	if err != nil {
		return nil, err
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
		return err
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
		return err
	}
	return err
}

func (handle kubeHandle) applyConfig(name string, config map[string]string) (*corev1.ConfigMap, error) {
	// FIXME Define builtin non-overridable configuration
	config["ENROLL_ADAPTER_KEY"] = name
	adapterConfig := coreapplyv1.ConfigMapApplyConfiguration{
		TypeMetaApplyConfiguration:   *metaapplyv1.TypeMeta().WithKind("ConfigMap").WithAPIVersion("v1"),
		ObjectMetaApplyConfiguration: metaapplyv1.ObjectMeta().WithName(name).WithNamespace(handle.nameSpace),
		Data:                         config,
	}
	configMap, err := handle.clientSet.CoreV1().ConfigMaps(handle.nameSpace).Apply(context.TODO(), &adapterConfig, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	return configMap, err
}

func (handle kubeHandle) applyDeployment(name string, image intermediaries.Image) (*appsv1.Deployment, *corev1.Service, error) {
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
		logging.Error("Error applying Deployment", map[string]interface{}{"ERROR": err.Error()})
		return nil, nil, err
	}
	servicePortSpec := coreapplyv1.ServicePort().WithAppProtocol("TCP").WithPort(8080).WithTargetPort(intstr.FromInt(8080))
	serviceSpec := coreapplyv1.ServiceSpec().WithSelector(podLabels).WithPorts(servicePortSpec)
	service := coreapplyv1.Service(name, handle.nameSpace).WithSpec(serviceSpec)

	appliedService, err := handle.clientSet.CoreV1().Services(handle.nameSpace).Apply(context.TODO(), service, metav1.ApplyOptions{FieldManager: "adapter-attendant"})
	return appliedDeployment, appliedService, err
}

func (handle kubeHandle) ApplyAdapter(adapter intermediaries.Adapter) (intermediaries.Adapter, error) {
	// FIXME This function is a piece of crap. I need to figure out a way to make this more REST-y while still;
	// * Preventing configuration being created without a deployment
	// * Preventing deployment from being created without configuration
	// * Allow deployment be updated (image name/tag) without supplying configuration
	// * Allow configuration be updated without supplying image information
	if adapter.Config != nil {
		// If config is supplied we should apply a ConfigMap
		_, err := handle.applyConfig(adapter.Name, adapter.Config)
		if err != nil {
			logging.Error("Error applying config map", map[string]interface{}{"ERROR": err.Error()})
			return intermediaries.Adapter{}, err
		}
	} else {
		// If we do not post config, we need to make sure we at least have one already present
		_, err := handle.clientSet.CoreV1().ConfigMaps(handle.nameSpace).Get(context.TODO(), adapter.Name, metav1.GetOptions{})
		if err != nil {
			logging.Error("No config, and none present", map[string]interface{}{"ERROR": err.Error()})
			return intermediaries.Adapter{}, err
		}
	}

	if adapter.Image != nil {
		// If image is set, we
		_, _, err := handle.applyDeployment(adapter.Name, *adapter.Image)
		if err != nil {
			return adapter, err
		}
	}
	return adapter, nil
}

func NewPureK8sBackend(conf config.Kubernetes) (AdapterAttendantDB, error) {
	// FIXME Do we want any other kind?
	kubeConf, err := clientcmd.BuildConfigFromFlags("", conf.KubeConfigPath)
	if err != nil {
		return nil, err
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
