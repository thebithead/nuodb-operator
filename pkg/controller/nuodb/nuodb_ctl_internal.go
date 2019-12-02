package nuodb

import (
	"context"
	"fmt"
	"github.com/fatih/structs"
	"github.com/mitchellh/mapstructure"
	ocpappsv1 "github.com/openshift/api/apps/v1"
	v12 "github.com/openshift/api/route/v1"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	cpb "k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/timeconv"
	tversion "k8s.io/helm/pkg/version"
	nuodbv2alpha1 "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	"nuodb/nuodb-operator/pkg/trace"
	"nuodb/nuodb-operator/pkg/utils"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

type NuoResource struct {
	name               string
	kind               string
	template           string
	templateFilename   string
	templateDecodedMap map[string]interface{}
	templateMetadata   map[string]interface{}
}

type NuoResources struct {
	values      map[string]string
	nuoResourceList []NuoResource
}

func nuoResourcesInit(instance *nuodbv2alpha1.Nuodb) (NuoResources, error) {
	var nuoResources NuoResources
	var err error = nil
	chartDir := utils.NuodbChartDir
	// TODO: when running in a docker container the base chartDir should be "/usr/local/etc/nuodb-operator/charts"
	nuoResources, err = processNuodbTemplates(chartDir, instance.Spec)
	if err != nil {
		log.Error(err, "Failed to process templates")
		return nuoResources, utils.ConvertError(err)
	}
	nuoResources.nuoResourceList = make([]NuoResource, 0)
	err = processNuoResources(&nuoResources)
	return nuoResources, err
}

func processNuodbTemplates(chartDir string, spec nuodbv2alpha1.NuodbSpec) (NuoResources, error) {
	var nuoResources NuoResources
	c, err := chartutil.Load(chartDir)
	if err != nil {
		log.Error(err,"Failed to process chart directory.")
		return nuoResources, err
	}

	options := chartutil.ReleaseOptions{Name: "nuodb-release", Time: timeconv.Now(), Namespace: "nuodb"}
	caps := &chartutil.Capabilities{
		APIVersions:   chartutil.DefaultVersionSet,
		KubeVersion:   chartutil.DefaultKubeVersion,
		TillerVersion: tversion.GetVersionProto(),
	}

	var mNew = make(map[string]*cpb.Value)
	st := reflect.TypeOf(spec)
	sv := reflect.ValueOf(&spec).Elem()
	for i := 0; i < st.NumField(); i++ {
		field := st.Field(i)
		fieldValue := sv.Field(i)
		fieldTag := field.Tag
		fieldTagString := fieldTag.Get("json")
		str := fmt.Sprintf("%v", fieldValue.Interface())
		cv := cpb.Value{Value:str}
		mNew[fieldTagString] = &cv
	}

	m := structs.Map(spec)
	var m2 = make(map[string]*cpb.Value)
	for k, v := range m {
		str := fmt.Sprintf("%v", v)
		cv := cpb.Value{Value:str}
		m2[k] = &cv
	}
	yvalues := chartutil.ToYaml(m2)
	vals := cpb.Config{Raw: yvalues}

	cvals, err := chartutil.CoalesceValues(c, &vals)
	if err != nil {
		log.Error(err,"Failed to coalesce values.")
		return nuoResources, err
	}
	for k, v := range mNew {
		str := fmt.Sprintf("%s", v.Value)
		cvals[k] = str
	}

	// convert our values back into config
	yvals, err := cvals.YAML()
	if err != nil {
		log.Error(err,"Failed to convert our values back into config.")
		return nuoResources, err
	}
	cc := &cpb.Config{Raw: yvals}
	valuesToRender, err := chartutil.ToRenderValuesCaps(c, cc, options, caps)
	if err != nil {
		log.Error(err,"Failed chartutil.ToRenderValuesCaps().")
		return nuoResources, err
	}
	e := engine.New()

	out, err := e.Render(c, valuesToRender)
	if err != nil {
		log.Error(err,"Failed to render templates.")
		return nuoResources, err
	}
	nuoResources.values = out
	return nuoResources, err
}

func getnuodbv2alpha1NuodbInstanceUsingClient(thisClient client.Client, request reconcile.Request) (*nuodbv2alpha1.Nuodb, error) {
	// Fetch the Nuodb instance
	nuodbv2alpha1NuodbInstance := &nuodbv2alpha1.Nuodb{}
	err := thisClient.Get(context.TODO(), request.NamespacedName, nuodbv2alpha1NuodbInstance)
	return nuodbv2alpha1NuodbInstance, err
}

func getnuodbv2alpha1NuodbInstance(r *ReconcileNuodb, request reconcile.Request) (*nuodbv2alpha1.Nuodb, error) {
	// Fetch the Nuodb instance
	return getnuodbv2alpha1NuodbInstanceUsingClient(r.client, request)
}

func createNuodbPod(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*corev1.Pod, error) {
	pod, err := utils.CreatePodFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return pod, err
}

func createNuodbService(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*corev1.Service, error) {
	service, err := utils.CreateServiceFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return service, err
}

func createNuodbSecret(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*corev1.Secret, error) {
	secret, err := utils.CreateSecretFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return secret, err
}

func createNuodbConfigMap(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*corev1.ConfigMap, error) {
	configMap, err := utils.CreateConfigMapFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return configMap, err
}

func createNuodbRoute(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*v12.Route, error) {
	route, err := utils.CreateRouteFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return route, err
}

func createNuodbReplicationController(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*corev1.ReplicationController, error) {
	replicationController, err := utils.CreateReplicationControllerFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return replicationController, err
}

func createNuodbDeployment(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*appsv1.Deployment, error) {
	deployment, err := utils.CreateDeploymentFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return deployment, err
}

func createNuodbDeploymentConfig(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*ocpappsv1.DeploymentConfig, error) {
	deploymentConfig, err := utils.CreateDeploymentConfigFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return deploymentConfig, err
}

func createNuodbStatefulSet(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*appsv1.StatefulSet, error) {
	statefulSet, err := utils.CreateStatefulSetFromTemplate(instance, thisClient, thisScheme,
		nuoResource.template, request.Namespace)
	return statefulSet, err
}

func createNuodbDaemonSet(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*appsv1.DaemonSet, error) {
	daemonSet, err := utils.CreateDaemonSetFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return daemonSet, err
}

func reconcileNuodbPod(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string)(*corev1.Pod, reconcile.Result, error) {
	var pod *corev1.Pod = nil
	pod, err := utils.GetPod(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			pod, err = createNuodbPod(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return pod, reconcile.Result{}, err
			}
		} else {
			return pod, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return pod, reconcile.Result{}, err
}

func reconcileNuodbService(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*corev1.Service, reconcile.Result, error) {
	var service *corev1.Service = nil
	service, err := utils.GetService(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			service, err = createNuodbService(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return service, reconcile.Result{}, err
			}
		} else {
			return service, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return service, reconcile.Result{}, err
}

func reconcileNuodbSecret(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*corev1.Secret, reconcile.Result, error) {
	var secret *corev1.Secret = nil
	secret, err := utils.GetSecret(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			secret, err = createNuodbSecret(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return secret, reconcile.Result{}, err
			}
		} else {
			return secret, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return secret, reconcile.Result{}, err
}

func reconcileNuodbConfigMap(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*corev1.ConfigMap, reconcile.Result, error) {
	var configMap *corev1.ConfigMap = nil
	configMap, err := utils.GetConfigMap(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			configMap, err = createNuodbConfigMap(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return configMap, reconcile.Result{}, err
			}
		} else {
			return configMap, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return configMap, reconcile.Result{}, err
}

func reconcileNuodbRoute(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*v12.Route, reconcile.Result, error) {
	var route *v12.Route = nil
	route, err := utils.GetRoute(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			route, err = createNuodbRoute(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return route, reconcile.Result{}, err
			}
		} else {
			return route, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return route, reconcile.Result{}, err
}

func reconcileNuodbReplicationController(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*corev1.ReplicationController, reconcile.Result, error) {
	var replicationController *corev1.ReplicationController = nil
	replicationController, err := utils.GetReplicationController(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			replicationController, err = createNuodbReplicationController(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return replicationController, reconcile.Result{}, err
			}
		} else {
			return replicationController, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return replicationController, reconcile.Result{}, err
}

func reconcileNuodbDeployment(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*appsv1.Deployment, reconcile.Result, error) {
	var deployment *appsv1.Deployment = nil
	deployment, err := utils.GetDeployment(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			deployment, err = createNuodbDeployment(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return deployment, reconcile.Result{}, err
			}
		} else {
			return deployment, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	} else {
		if nuoResource.name == "te" {
			_, _, err = updateTeReadyCount(thisClient, request, deployment.Status.ReadyReplicas)
			if err != nil {
				log.Error(err, "Error: Unable to update TE ready count.")
				return deployment, reconcile.Result{}, trace.Wrap(err)
			}
			if *deployment.Spec.Replicas != instance.Spec.TeCount {
				*deployment.Spec.Replicas = instance.Spec.TeCount
				err = thisClient.Update(context.TODO(), deployment)
				if err != nil {
					log.Error(err, "Error: Unable to update TeCount in TE Deployment.")
					return deployment, reconcile.Result{}, trace.Wrap(err)
				}
			}
		}
	}
	return deployment, reconcile.Result{}, err
}

func reconcileNuodbDeploymentConfig(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*ocpappsv1.DeploymentConfig, reconcile.Result, error) {
	var deploymentConfig *ocpappsv1.DeploymentConfig = nil
	deploymentConfig, err := utils.GetDeploymentConfig(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			deploymentConfig, err = createNuodbDeploymentConfig(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return deploymentConfig, reconcile.Result{}, err
			}
		} else {
			return deploymentConfig, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	} else {
		if nuoResource.name == "te" {
			_, _, err = updateTeReadyCount(thisClient, request, deploymentConfig.Status.ReadyReplicas)
			if err != nil {
				log.Error(err, "Error: Unable to update TE ready count.")
				return deploymentConfig, reconcile.Result{}, trace.Wrap(err)
			}
			if deploymentConfig.Spec.Replicas != instance.Spec.TeCount {
				deploymentConfig.Spec.Replicas = instance.Spec.TeCount
				err = thisClient.Update(context.TODO(), deploymentConfig)
				if err != nil {
					log.Error(err, "Error: Unable to update TeCount in TE DeploymentConfig.")
					return deploymentConfig, reconcile.Result{}, trace.Wrap(err)
				}
			}
		}
	}
	return deploymentConfig, reconcile.Result{}, err
}

func reconcileNuodbStatefulSet(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*appsv1.StatefulSet, reconcile.Result, error) {
	var statefulSet *appsv1.StatefulSet = nil
	statefulSet, err := utils.GetStatefulSet(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			statefulSet, err = createNuodbStatefulSet(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return statefulSet, reconcile.Result{}, err
			}
		} else {
			return statefulSet, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	} else {
		if nuoResource.name == "admin" {
			_, _, err = updateAdminReadyCount(thisClient, request, statefulSet.Status.ReadyReplicas)
			if err != nil {
				log.Error(err, "Error: Unable to update Admin ready count.")
				return statefulSet, reconcile.Result{}, trace.Wrap(err)
			}
			if *statefulSet.Spec.Replicas != instance.Spec.AdminCount {
				*statefulSet.Spec.Replicas = instance.Spec.AdminCount
				err = thisClient.Update(context.TODO(), statefulSet)
				if err != nil {
					log.Error(err, "Error: Unable to update AdminCount in Admin StatefulSet.")
					return statefulSet, reconcile.Result{}, trace.Wrap(err)
				}
			}
		} else if nuoResource.name == "sm" {
			_, _, err = updateSmReadyCount(thisClient, request, statefulSet.Status.ReadyReplicas)
			if err != nil {
				log.Error(err, "Error: Unable to update SM ready count.")
				return statefulSet, reconcile.Result{}, trace.Wrap(err)
			}
			if *statefulSet.Spec.Replicas != instance.Spec.SmCount {
				*statefulSet.Spec.Replicas = instance.Spec.SmCount
				err = thisClient.Update(context.TODO(), statefulSet)
				if err != nil {
					log.Error(err, "Error: Unable to update SmCount in SM StatefulSet.")
					return statefulSet, reconcile.Result{}, trace.Wrap(err)
				}
			}
		}

	}
	return statefulSet, reconcile.Result{}, err
}

func reconcileNuodbDaemonSet(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*appsv1.DaemonSet, reconcile.Result, error) {
	var daemonSet *appsv1.DaemonSet = nil
	daemonSet, err := utils.GetDaemonSet(thisClient, namespace, nuoResource.name)
	if err != nil {
		sErr, ok := err.(*apierrors.StatusError)
		if ok && sErr.Status().Reason == "NotFound"{
			daemonSet, err = createNuodbDaemonSet(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return daemonSet, reconcile.Result{}, err
			}
		} else {
			return daemonSet, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return daemonSet, reconcile.Result{}, err
}

func processNuoResources(nuoResources *NuoResources) error {
	for templateFilename, template := range (*nuoResources).values {
		var nuoResource NuoResource
		nuoResource.template = template
		nuoResource.templateFilename = templateFilename
		m := make(map[interface{}]interface{})
		if err := yaml.Unmarshal([]byte(template), &m); err != nil {
			log.Error(err, "Error unmarshaling the YAML byte stream.")
			return utils.ConvertError(err)
		}
		var m2 map[string]interface{}
		if err := mapstructure.Decode(m, &m2); err != nil {
			log.Error(err, "Error mapstructure.Decode().")
			return utils.ConvertError(err)
		}
		nuoResource.templateDecodedMap = m2
		nuoResource.kind = m2["kind"].(string)
		var metadata map[string]interface{}
		if err := mapstructure.Decode(m2["metadata"], &metadata); err != nil {
			log.Error(err, "Error mapstructure.Decode().")
			return utils.ConvertError(err)
		}
		nuoResource.name = metadata["name"].(string)
		nuoResource.templateMetadata = metadata
		(*nuoResources).nuoResourceList = append((*nuoResources).nuoResourceList, nuoResource)
	}
	return nil
}

func updateStatus(thisClient client.Client, request reconcile.Request, status nuodbv2alpha1.NuodbStatus) (*nuodbv2alpha1.Nuodb, bool, error) {
	// Fetch the Nuodb instance
	currentInstance, err := getnuodbv2alpha1NuodbInstanceUsingClient(thisClient, request)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil, false, nil
		}
		// Error reading the object - requeue the request.
		return nil, false, err
	}
	// Update Admin Health
	if status.AdminReadyCount >= currentInstance.Spec.AdminCount {
		status.AdminHealth = nuodbv2alpha1.NuodbGreenHealth
	} else if status.AdminReadyCount == 0 {
		status.AdminHealth = nuodbv2alpha1.NuodbRedHealth
	} else {
		status.AdminHealth = nuodbv2alpha1.NuodbYellowHealth
	}

	// Update SM Health
	if status.SmReadyCount >= currentInstance.Spec.SmCount {
		status.SmHealth = nuodbv2alpha1.NuodbGreenHealth
	} else if status.SmReadyCount == 0 {
		status.SmHealth = nuodbv2alpha1.NuodbRedHealth
	} else {
		status.SmHealth = nuodbv2alpha1.NuodbYellowHealth
	}

	// Update TE Health
	if status.TeReadyCount >= currentInstance.Spec.TeCount {
		status.TeHealth = nuodbv2alpha1.NuodbGreenHealth
	} else if status.TeReadyCount == 0 {
		status.TeHealth = nuodbv2alpha1.NuodbRedHealth
	} else {
		status.TeHealth = nuodbv2alpha1.NuodbYellowHealth
	}

	// Derive Domain Health from Admin/SM/TE Health
	if ((status.TeHealth == nuodbv2alpha1.NuodbGreenHealth) &&
		(status.SmHealth == nuodbv2alpha1.NuodbGreenHealth) &&
		(status.AdminHealth == nuodbv2alpha1.NuodbGreenHealth)) {
		status.DomainHealth = nuodbv2alpha1.NuodbGreenHealth
		status.Phase = 	nuodbv2alpha1.NuodbOperationalPhase
	} else if ((status.TeHealth == nuodbv2alpha1.NuodbRedHealth) ||
		(status.SmHealth == nuodbv2alpha1.NuodbRedHealth) ||
		(status.AdminHealth == nuodbv2alpha1.NuodbRedHealth)) {
		status.DomainHealth = nuodbv2alpha1.NuodbRedHealth
		status.Phase = 	nuodbv2alpha1.NuodbPendingPhase
	} else {
		status.DomainHealth = nuodbv2alpha1.NuodbYellowHealth
		status.Phase = 	nuodbv2alpha1.NuodbOperationalPhase
	}

	if !reflect.DeepEqual(currentInstance.Status, status) {
		status.DeepCopyInto(&currentInstance.Status)
		err = thisClient.Update(context.TODO(), currentInstance)
		if err != nil {
			return nil, false, err
		}
		return currentInstance, true, err
	}
	return currentInstance, false, err
}

func updateAdminReadyCount(thisClient client.Client, request reconcile.Request,
	adminReadyCount int32) (*nuodbv2alpha1.Nuodb, bool, error) {
	// Fetch the Nuodb instance
	currentInstance, err := getnuodbv2alpha1NuodbInstanceUsingClient(thisClient, request)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil, false, nil
		}
		// Error reading the object - requeue the request.
		return nil, false, err
	}
	if currentInstance.Status.AdminReadyCount == adminReadyCount {
		return currentInstance, false, nil
	}
	newStatus := nuodbv2alpha1.NuodbStatus{}
	currentInstance.Status.DeepCopyInto(&newStatus)
	newStatus.AdminReadyCount = adminReadyCount
	return updateStatus(thisClient, request, newStatus)
}

func updateSmReadyCount(thisClient client.Client, request reconcile.Request,
	smReadyCount int32) (*nuodbv2alpha1.Nuodb, bool, error) {
	// Fetch the Nuodb instance
	currentInstance, err := getnuodbv2alpha1NuodbInstanceUsingClient(thisClient, request)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil, false, nil
		}
		// Error reading the object - requeue the request.
		return nil, false, err
	}
	if currentInstance.Status.SmReadyCount == smReadyCount {
		return currentInstance, false, nil
	}
	newStatus := nuodbv2alpha1.NuodbStatus{}
	currentInstance.Status.DeepCopyInto(&newStatus)
	newStatus.SmReadyCount = smReadyCount
	return updateStatus(thisClient, request, newStatus)
}

func updateTeReadyCount(thisClient client.Client, request reconcile.Request,
	teReadyCount int32) (*nuodbv2alpha1.Nuodb, bool, error) {
	// Fetch the Nuodb instance
	currentInstance, err := getnuodbv2alpha1NuodbInstanceUsingClient(thisClient, request)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil, false, nil
		}
		// Error reading the object - requeue the request.
		return nil, false, err
	}
	if currentInstance.Status.TeReadyCount == teReadyCount {
		return currentInstance, false, nil
	}
	newStatus := nuodbv2alpha1.NuodbStatus{}
	currentInstance.Status.DeepCopyInto(&newStatus)
	newStatus.TeReadyCount = teReadyCount
	return updateStatus(thisClient, request, newStatus)
}


func reconcileNuodbInternal(r *ReconcileNuodb, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Nuodb")

	// Fetch the Nuodb instance
	instance, err := getnuodbv2alpha1NuodbInstance(r, request)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	nuodbStatus := nuodbv2alpha1.NuodbStatus{
		ControllerVersion: utils.NuodbOperatorVersion,
		Phase:             nuodbv2alpha1.NuodbPendingPhase,
		AdminReadyCount:   0,
		SmReadyCount:      0,
		TeReadyCount:      0,
		AdminHealth:       nuodbv2alpha1.NuodbUnknownHealth,
		SmHealth:          nuodbv2alpha1.NuodbUnknownHealth,
		TeHealth:          nuodbv2alpha1.NuodbUnknownHealth,
		DomainHealth:      nuodbv2alpha1.NuodbUnknownHealth,
	}

	if instance.Status.ControllerVersion == "" {
		_, _, err = updateStatus(r.client, request, nuodbStatus)
		return reconcile.Result{Requeue:true}, err
	}

	nuoResources, err := nuoResourcesInit(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	processOrder := [] string {
	"Secret",
	"ConfigMap",
	"Service",
	"Route",
	"Pod",
	"ReplicationController",
	"Deployment",
	"DeploymentConfig",
	"StatefulSet",
	"DaemonSet"}

	currentTime := time.Now()
	log.Info("Starting Reconcile request: " + currentTime.String())
	var rr reconcile.Result

	for item := range processOrder {
		for _, nuoResource := range nuoResources.nuoResourceList {
			if nuoResource.kind == processOrder[item] {
				switch nuoResource.kind {
				case "Pod":
					_, rr, err := reconcileNuodbPod(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue  {
						return rr, err
					}
				case "Service":
					_, rr, err = reconcileNuodbService(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				case "Secret":
					_, rr, err = reconcileNuodbSecret(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				case "ConfigMap":
					_, rr, err = reconcileNuodbConfigMap(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				case "ReplicationController":
					_, rr, err = reconcileNuodbReplicationController(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				case "Deployment":
					_, rr, err = reconcileNuodbDeployment(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				case "StatefulSet":
					_, rr, err = reconcileNuodbStatefulSet(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				case "DaemonSet":
					_, rr, err = reconcileNuodbDaemonSet(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				}
			}
		}
	}
	return reconcile.Result{RequeueAfter:time.Duration(10) * time.Second}, nil
}
