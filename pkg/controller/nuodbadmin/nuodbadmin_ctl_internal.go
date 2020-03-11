// This is the K8s Controller for K8s Kind nuodbadmin.
// All of the reconcile functions have the name prefix: reconcileNuodbAdmin

package nuodbadmin

import (
	"context"
	"fmt"
	"github.com/fatih/structs"
	"github.com/mitchellh/mapstructure"
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
	"strings"
	//logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
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

func nuoAdminResourcesInit(instance *nuodbv2alpha1.NuodbAdmin) (NuoResources, error) {
	var nuoResources NuoResources
	var err error = nil
	chartDir := utils.NuodbAdminChartDir
	// TODO: when running in a docker container the base chartDir should be "/usr/local/etc/nuodb-operator/charts"
	nuoResources, err = processNuodbAdminTemplates(chartDir, instance.Spec)
	if err != nil {
		log.Error(err, "Failed to process templates")
		return nuoResources, utils.ConvertError(err)
	}
	nuoResources.nuoResourceList = make([]NuoResource, 0)
	err = processNuoResources(&nuoResources)
	return nuoResources, err
}

func processNuodbAdminTemplates(chartDir string, spec nuodbv2alpha1.NuodbAdminSpec) (NuoResources, error) {
	var nuoResources NuoResources
	c, err := chartutil.Load(chartDir)
	if err != nil {
		log.Error(err,"Failed to process chart directory.")
		return nuoResources, err
	}

	options := chartutil.ReleaseOptions{Name: "nuodbadmin-release", Time: timeconv.Now(), Namespace: "nuodb"}
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

func getnuodbv2alpha1NuodbAdminInstanceUsingClient(thisClient client.Client, request reconcile.Request) (*nuodbv2alpha1.NuodbAdmin, error) {
	// Fetch the Nuodb instance
	nuodbv2alpha1NuodbAdminInstance := &nuodbv2alpha1.NuodbAdmin{}
	err := thisClient.Get(context.TODO(), request.NamespacedName, nuodbv2alpha1NuodbAdminInstance)
	return nuodbv2alpha1NuodbAdminInstance, err
}

func getnuodbv2alpha1NuodbAdminInstance(r *ReconcileNuodbAdmin, request reconcile.Request) (*nuodbv2alpha1.NuodbAdmin, error) {
	// Fetch the Nuodb instance
	return getnuodbv2alpha1NuodbAdminInstanceUsingClient(r.client, request)
}

func createNuodbAdminPod(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbAdmin,
	nuoResource NuoResource) (*corev1.Pod, error) {
	pod, err := utils.CreatePodFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return pod, err
}

func createNuodbAdminService(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbAdmin,
	nuoResource NuoResource) (*corev1.Service, error) {
	service, err := utils.CreateServiceFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return service, err
}

func createNuodbAdminConfigMap(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbAdmin,
	nuoResource NuoResource) (*corev1.ConfigMap, error) {
	configMap, err := utils.CreateConfigMapFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return configMap, err
}

func createNuodbAdminStatefulSet(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbAdmin,
	nuoResource NuoResource) (*appsv1.StatefulSet, error) {
	statefulSet, err := utils.CreateStatefulSetFromTemplate(instance, thisClient, thisScheme,
		nuoResource.template, request.Namespace, nuoResource.name)
	return statefulSet, err
}

func reconcileNuodbAdminPod(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbAdmin,
	nuoResource NuoResource, namespace string)(*corev1.Pod, reconcile.Result, error) {
	var pod *corev1.Pod = nil
	var isInsightsResource = strings.HasSuffix(nuoResource.templateFilename, "-insights.yaml")

	pod, err := utils.GetPod(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if isInsightsResource && !instance.Spec.InsightsEnabled {
				return nil, reconcile.Result{}, nil
			}
			pod, err = createNuodbAdminPod(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return pod, reconcile.Result{}, err
			}
		} else {
			return pod, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	} else {
		if isInsightsResource && !instance.Spec.InsightsEnabled {
			log.Info("Hosted NuoDB Insights disabled - Deleting Pod:",
				"Namespace", request.Namespace, "Name", nuoResource.name)
			err = thisClient.Delete(context.TODO(), pod)
			return nil, reconcile.Result{}, err
		}
	}
	return pod, reconcile.Result{}, err
}

func reconcileNuodbAdminService(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbAdmin,
	nuoResource NuoResource, namespace string) (*corev1.Service, reconcile.Result, error) {
	var service *corev1.Service = nil
	var isInsightsResource = strings.HasSuffix(nuoResource.templateFilename, "-insights.yaml")

	service, err := utils.GetService(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if isInsightsResource && !instance.Spec.InsightsEnabled {
				return nil, reconcile.Result{}, nil
			}
			service, err = createNuodbAdminService(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return service, reconcile.Result{}, err
			}
		} else {
			return service, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	} else {
		if isInsightsResource && !instance.Spec.InsightsEnabled {
			log.Info("Hosted NuoDB Insights disabled - Deleting Service:",
				"Namespace", request.Namespace, "Name", nuoResource.name)
			err = thisClient.Delete(context.TODO(), service)
			return nil, reconcile.Result{}, err
		}
	}
	return service, reconcile.Result{}, err
}

func reconcileNuodbAdminConfigMap(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbAdmin,
	nuoResource NuoResource, namespace string) (*corev1.ConfigMap, reconcile.Result, error) {
	var configMap *corev1.ConfigMap = nil

	configMap, err := utils.GetConfigMap(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			configMap, err = createNuodbAdminConfigMap(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return configMap, reconcile.Result{}, err
			}
		} else {
			return configMap, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return configMap, reconcile.Result{}, err
}

func reconcileNuodbAdminStatefulSet(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbAdmin,
	nuoResource NuoResource, namespace string) (*appsv1.StatefulSet, reconcile.Result, error) {
	var statefulSet *appsv1.StatefulSet = nil
	var stsName = nuoResource.name
	var desiredAdminPodCount int32 = instance.Spec.AdminCount
	var err error = nil

	statefulSet, err = utils.GetStatefulSetV1(thisClient, namespace, stsName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			statefulSet, err = createNuodbAdminStatefulSet(thisClient, thisScheme, request, instance, nuoResource)
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
				if apierrors.IsConflict(err) {
					return statefulSet, reconcile.Result{}, err
				} else {
					log.Error(err, "Error: Unable to update Admin ready count.")
					return statefulSet, reconcile.Result{}, trace.Wrap(err)
				}
			}
			if *statefulSet.Spec.Replicas != desiredAdminPodCount {
				*statefulSet.Spec.Replicas = desiredAdminPodCount
				err = thisClient.Update(context.TODO(), statefulSet)
				if err != nil {
					if apierrors.IsConflict(err) {
						return statefulSet, reconcile.Result{}, err
					} else {
						log.Error(err, "Error: Unable to update AdminCount in Admin StatefulSet.")
						return statefulSet, reconcile.Result{}, trace.Wrap(err)
					}
				}
			}
		}
	}
	return statefulSet, reconcile.Result{}, err
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
func updateStatus(thisClient client.Client, request reconcile.Request, status nuodbv2alpha1.NuodbAdminStatus) (*nuodbv2alpha1.NuodbAdmin, bool, error) {
	// Fetch the Nuodb instance
	currentInstance, err := getnuodbv2alpha1NuodbAdminInstanceUsingClient(thisClient, request)
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

	// Derive Domain Health from Admin/SM/TE Health
	// TODO: Get status for each nuodb CR.
	if (status.AdminHealth == nuodbv2alpha1.NuodbGreenHealth) {
		status.DomainHealth = nuodbv2alpha1.NuodbGreenHealth
		status.Phase = 	nuodbv2alpha1.NuodbOperationalPhase
	} else if (status.AdminHealth == nuodbv2alpha1.NuodbRedHealth) {
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
	adminReadyCount int32) (*nuodbv2alpha1.NuodbAdmin, bool, error) {
	// Fetch the Nuodb instance
	currentInstance, err := getnuodbv2alpha1NuodbAdminInstanceUsingClient(thisClient, request)
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
	newStatus := nuodbv2alpha1.NuodbAdminStatus{}
	currentInstance.Status.DeepCopyInto(&newStatus)
	newStatus.AdminReadyCount = adminReadyCount
	return updateStatus(thisClient, request, newStatus)
}

func reconcileNuodbAdminInternal(r *ReconcileNuodbAdmin, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling NuodbAdmin")

	// Fetch the Nuodb instance
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, request)
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

	nuodbStatus := nuodbv2alpha1.NuodbAdminStatus{
		ControllerVersion: utils.NuodbOperatorVersion,
		Phase:             nuodbv2alpha1.NuodbPendingPhase,
		AdminReadyCount:   0,
		AdminHealth:       nuodbv2alpha1.NuodbUnknownHealth,
		DomainHealth:      nuodbv2alpha1.NuodbUnknownHealth,
	}

	if instance.Status.ControllerVersion == "" {
		_, _, err = updateStatus(r.client, request, nuodbStatus)
		return reconcile.Result{Requeue:true}, err
	}

	nuoResources, err := nuoAdminResourcesInit(instance)
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
					_, rr, err := reconcileNuodbAdminPod(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue  {
						return rr, err
					}
				case "Service":
					_, rr, err = reconcileNuodbAdminService(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				case "ConfigMap":
					_, rr, err = reconcileNuodbAdminConfigMap(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				case "StatefulSet":
					_, rr, err = reconcileNuodbAdminStatefulSet(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				}
			}
		}
	}
	return reconcile.Result{RequeueAfter:time.Duration(10) * time.Second}, nil
}
