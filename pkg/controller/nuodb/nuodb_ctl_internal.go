// This is the K8s Controller for K8s Kind nuodb.
// All of the reconcile functions have the name prefix: reconcileNuodb

package nuodb

import (
	"context"
	"fmt"
	"github.com/fatih/structs"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func createNuodbReplicationController(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*corev1.ReplicationController, error) {
	replicationController, err := utils.CreateReplicationControllerFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return replicationController, err
}

func createNuodbDeployment(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*appsv1.Deployment, error) {
	name := nuoResource.name
	if name == "te" {
		name = instance.Name + "-te"
	}
	deployment, err := utils.CreateDeploymentFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace, name)
	return deployment, err
}

func createNuodbStatefulSet(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource) (*appsv1.StatefulSet, error) {
	name := nuoResource.name
	if name == "sm" {
		name = instance.Name + "-sm"
	}
	statefulSet, err := utils.CreateStatefulSetFromTemplate(instance, thisClient, thisScheme,
		nuoResource.template, request.Namespace, name)
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

func reconcileNuodbDeployment(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*appsv1.Deployment, reconcile.Result, error) {
	var deployment *appsv1.Deployment = nil
	var teName = nuoResource.name
	if nuoResource.name == "te" {
		teName = request.Name + "-te"
	}
	deployment, err := utils.GetDeployment(thisClient, namespace, teName)
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

func reconcileNuodbStatefulSet(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.Nuodb,
	nuoResource NuoResource, namespace string) (*appsv1.StatefulSet, reconcile.Result, error) {
	var statefulSet *appsv1.StatefulSet = nil
	var stsName = nuoResource.name
	if nuoResource.name == "sm" {
		stsName = request.Name + "-sm"
	}

	list := &nuodbv2alpha1.NuodbList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "nuodbs.nuodb.com/v2alpha1",
			Kind:       "Nuodb",
		},
	}
	listOpts := client.ListOptions{
		Namespace:     request.Namespace,
	}

	err := thisClient.List(context.TODO(), &listOpts, list)
	if err != nil {
		panic(err)
	}
	statefulSet, err = utils.GetStatefulSetV1(thisClient, namespace, stsName)
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
		if nuoResource.name == "sm" {
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

//noinspection GoRedundantParens
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

	// Update SM Health
	if status.SmReadyCount >= currentInstance.Spec.SmCount {
		status.SmHealth = utils.NuodbGreenHealth
	} else if status.SmReadyCount == 0 {
		status.SmHealth = utils.NuodbRedHealth
	} else {
		status.SmHealth = utils.NuodbYellowHealth
	}

	// Update TE Health
	if status.TeReadyCount >= currentInstance.Spec.TeCount {
		status.TeHealth = utils.NuodbGreenHealth
	} else if status.TeReadyCount == 0 {
		status.TeHealth = utils.NuodbRedHealth
	} else {
		status.TeHealth = utils.NuodbYellowHealth
	}

	// Get Phase
	if ((status.TeHealth == utils.NuodbGreenHealth) &&
		(status.SmHealth == utils.NuodbGreenHealth)) {
		status.DatabaseHealth = utils.NuodbGreenHealth
		status.Phase = 	utils.NuodbOperationalPhase
	} else if ((status.TeHealth == utils.NuodbRedHealth) ||
		(status.SmHealth == utils.NuodbRedHealth)) {
		status.DatabaseHealth = utils.NuodbRedHealth
		status.Phase = 	utils.NuodbPendingPhase
	} else {
		status.DatabaseHealth = utils.NuodbYellowHealth
		status.Phase = 	utils.NuodbOperationalPhase
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
		Phase:             utils.NuodbPendingPhase,
		SmReadyCount:      0,
		TeReadyCount:      0,
		SmHealth:          utils.NuodbUnknownHealth,
		TeHealth:          utils.NuodbUnknownHealth,
		DatabaseHealth:    utils.NuodbUnknownHealth,
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
				case "Secret":
					_, rr, err = reconcileNuodbSecret(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
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
				case "ConfigMap":
					_, rr, err = reconcileNuodbConfigMap(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				}

			}
		}
	}
	return reconcile.Result{RequeueAfter:time.Duration(10) * time.Second}, nil
}
