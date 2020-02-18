// This is the K8s Controller for K8s Kind nuodbycsbwl.
// All of the reconcile functions have the name prefix: reconcileNuodbYcsbWl

package nuodbycsbwl

import (
	"context"
	"fmt"
	"github.com/fatih/structs"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"
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

type NuoYcsbWlResource struct {
	name               string
	kind               string
	template           string
	templateFilename   string
	templateDecodedMap map[string]interface{}
	templateMetadata   map[string]interface{}
}

type NuoYcsbWlResources struct {
	values                map[string]string
	nuoYcsbWlResourceList []NuoYcsbWlResource
}

func nuoYcsbWlResourcesInit(instance *nuodbv2alpha1.NuodbYcsbWl) (NuoYcsbWlResources, error) {
	var nuoYcsbWlResources NuoYcsbWlResources
	var err error = nil
	chartDir := utils.NuodbYcsbChartDir
	nuoYcsbWlResources, err = processNuodbYcsbWlTemplates(chartDir, instance.Spec)
	if err != nil {
		log.Error(err, "Failed to process templates")
		return nuoYcsbWlResources, utils.ConvertError(err)
	}
	nuoYcsbWlResources.nuoYcsbWlResourceList = make([]NuoYcsbWlResource, 0)
	err = processNuodbYcsbWlResources(&nuoYcsbWlResources)
	return nuoYcsbWlResources, err
}

func processNuodbYcsbWlTemplates(chartDir string, spec nuodbv2alpha1.NuodbYcsbWlSpec) (NuoYcsbWlResources, error) {
	var nuoYcsbWlResources NuoYcsbWlResources
	c, err := chartutil.Load(chartDir)
	if err != nil {
		log.Error(err,"Failed to process chart directory.")
		return nuoYcsbWlResources, err
	}

	options := chartutil.ReleaseOptions{Name: "nuodbycsbwl-release", Time: timeconv.Now(), Namespace: "nuodb"}
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
		return nuoYcsbWlResources, err
	}
	for k, v := range mNew {
		str := fmt.Sprintf("%s", v.Value)
		cvals[k] = str
	}

	// convert our values back into config
	yvals, err := cvals.YAML()
	if err != nil {
		log.Error(err,"Failed to convert our values back into config.")
		return nuoYcsbWlResources, err
	}
	cc := &cpb.Config{Raw: yvals}
	valuesToRender, err := chartutil.ToRenderValuesCaps(c, cc, options, caps)
	if err != nil {
		log.Error(err,"Failed chartutil.ToRenderValuesCaps().")
		return nuoYcsbWlResources, err
	}
	e := engine.New()

	out, err := e.Render(c, valuesToRender)
	if err != nil {
		log.Error(err,"Failed to render templates.")
		return nuoYcsbWlResources, err
	}
	nuoYcsbWlResources.values = out
	return nuoYcsbWlResources, err
}

func getnuodbv2alpha1NuodbYcsbWlInstance(r *ReconcileNuodbYcsbWl, request reconcile.Request) (*nuodbv2alpha1.NuodbYcsbWl, error) {
	// Fetch the Nuodb instance
	nuodbv2alpha1NuodbycsbwlInstance := &nuodbv2alpha1.NuodbYcsbWl{}
	err := r.client.Get(context.TODO(), request.NamespacedName, nuodbv2alpha1NuodbycsbwlInstance)
    return nuodbv2alpha1NuodbycsbwlInstance, err
}

func createNuodbYcsbWlReplicationController(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbYcsbWl,
	nuoResource NuoYcsbWlResource) (*corev1.ReplicationController, error) {
	replicationController, err := utils.CreateReplicationControllerFromTemplate(instance, thisClient, thisScheme, nuoResource.template, request.Namespace)
	return replicationController, err
}

func reconcileNuodbYcsbWlReplicationController(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbYcsbWl,
	nuoResource NuoYcsbWlResource, namespace string) (*corev1.ReplicationController, reconcile.Result, error) {
	var replicationController *corev1.ReplicationController = nil
	replicationController, err := utils.GetReplicationController(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			replicationController, err = createNuodbYcsbWlReplicationController(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return replicationController, reconcile.Result{}, err
			}
		} else {
			return replicationController, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	} else {
		if nuoResource.name == "ycsb-load" {
			if *replicationController.Spec.Replicas != instance.Spec.YcsbWorkloadCount {
				*replicationController.Spec.Replicas = instance.Spec.YcsbWorkloadCount
				err = thisClient.Update(context.TODO(), replicationController)
				if err != nil {
					log.Error(err, "Error: ycsb-load replicationController Update.")
					return replicationController, reconcile.Result{}, trace.Wrap(err)
				}
			}
		}
	}
	return replicationController, reconcile.Result{}, err
}

func processNuodbYcsbWlResources(nuoResources *NuoYcsbWlResources) error {
	for templateFilename, template := range (*nuoResources).values {
		var nuoResource NuoYcsbWlResource
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
		(*nuoResources).nuoYcsbWlResourceList = append((*nuoResources).nuoYcsbWlResourceList, nuoResource)
	}
	return nil
}

func reconcileNuodbYcsbWlInternal(r *ReconcileNuodbYcsbWl, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling NuodbYcsbWl")

	// Fetch the NuodbYcsbWl instance
	instance, err := getnuodbv2alpha1NuodbYcsbWlInstance(r, request)
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

	nuoResources, err := nuoYcsbWlResourcesInit(instance)
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
		"StatefulSet" }

	currentTime := time.Now()
	log.Info("Starting Reconcile request: " + currentTime.String())
	var rr reconcile.Result

	for item := range processOrder {
		for _, nuoResource := range nuoResources.nuoYcsbWlResourceList {
			if nuoResource.kind == processOrder[item] {
				switch nuoResource.kind {
				case "ReplicationController":
					_, rr, err = reconcileNuodbYcsbWlReplicationController(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue {
						return rr, err
					}
				default:
					msg := fmt.Sprintf("NuodbYcsbWl invalid resource kind: %s", nuoResource.kind)
					err = apierrors.NewBadRequest(msg)
					log.Error(err, msg)
					return reconcile.Result{}, err
				}
			}
		}
	}
	return reconcile.Result{}, nil
}
