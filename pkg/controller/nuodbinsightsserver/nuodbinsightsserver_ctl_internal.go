package nuodbinsightsserver

import (
	"context"
	"fmt"
	commonv1alpha1 "github.com/elastic/cloud-on-k8s/operators/pkg/apis/common/v1alpha1"
	"github.com/elastic/cloud-on-k8s/operators/pkg/apis/elasticsearch/v1alpha1"
	esv1alpha1 "github.com/elastic/cloud-on-k8s/operators/pkg/apis/elasticsearch/v1alpha1"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/fatih/structs"
	grafanav1alpha1 "github.com/integr8ly/grafana-operator/pkg/apis/integreatly/v1alpha1"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	pv1b1 "k8s.io/api/policy/v1beta1"
	rbacv12 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ptv1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/engine"
	cpb "k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/timeconv"
	tversion "k8s.io/helm/pkg/version"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/policy"
	nuodbv2alpha1 "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	"nuodb/nuodb-operator/pkg/trace"
	"nuodb/nuodb-operator/pkg/utils"
	"os"
	"path"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

func reconcileNuodbInsightsServerInternal(r *ReconcileNuodbInsightsServer, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling NuodbInsightsServer")

	// Fetch the NuodbInsightsServer instance
	instance := &nuodbv2alpha1.NuodbInsightsServer{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err,"Reconcile NuodbInsightsServer Failed: Unable to read NuodbInsightsServer object.", "Name", request.Name, "Namespace", request.Namespace)
		return reconcile.Result{}, err
	}

	rm, err := utils.GetNewRestMapper()
	if err != nil {
		return reconcile.Result{}, err
	}

	// Finalizers
	// Determine if instance is under deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !utils.ContainsString(instance.ObjectMeta.Finalizers, utils.ECKFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, utils.ECKFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				log.Error(err,"Reconcile NuodbInsightsServer Failed: Unable to add finalizer " +
					"to NuodbInsightsServer object.",
					"Name", request.Name, "Namespace", request.Namespace)
				return reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if utils.ContainsString(instance.ObjectMeta.Finalizers, utils.ECKFinalizerName) {
			// our finalizer is present, so let's delete any external dependencies
			if err := r.deleteExternalResources(instance, request, rm); err != nil {
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				log.Error(err,"Reconcile NuodbInsightsServer failed: Unable to delete external " +
					"resources for NuodbInsightsServer object.",
					"Name", request.Name, "Namespace", request.Namespace)
				return reconcile.Result{}, err
			}

			// remove our finalizer from the list and update it.
			instance.ObjectMeta.Finalizers = utils.RemoveString(instance.ObjectMeta.Finalizers, utils.ECKFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				log.Error(err,"Reconcile NuodbInsightsServer failed: Unable to update NuodbInsightsServer object.",
					"Name", request.Name, "Namespace", request.Namespace)
				return reconcile.Result{}, err
			}
		}
		log.Info("Reconcile NuodbInsightsServer Successful.",
			"Name", request.Name, "Namespace", request.Namespace)
		return reconcile.Result{}, nil
	}

	result, err := reconcileECKAllInOne(r.client, r.scheme, instance,
		utils.ECKAllInOneYamlFile, utils.ElasticNamespace)
	if err != nil {
		log.Error(err,"Reconcile NuodbInsightsServer failed: Failed reconcileECKAllInOne().",
			"Name", request.Name, "Namespace", request.Namespace)
		return result, err
	}

	result, err = reconcileESCluster(r, request, instance, utils.ESClusterName, rm)
	if err != nil {
		log.Error(err,"Reconcile NuodbInsightsServer failed: Failed reconcileESCluster().",
			"Name", request.Name, "Namespace", request.Namespace)
		return result, err
	}

	result, err = reconcileKibanaCluster(r, request, instance, utils.KibanaYamlFile, utils.KibanaClusterName, rm)
	if err != nil {
		log.Error(err,"Reconcile NuodbInsightsServer failed: Failed reconcileKibanaCluster().",
			"Name", request.Name, "Namespace", request.Namespace)
		return result, err
	}

	esClient, err := utils.GetESClient(request.Namespace)
	if err != nil {
		log.Error(err,"Reconcile NuodbInsightsServer failed: Failed utils.GetESClient().",
			"Name", request.Name, "Namespace", request.Namespace)
		return reconcile.Result{}, err
	}
	result, err = reconcileESTemplates(request, utils.ESClusterName, esClient)
	if err != nil {
		log.Error(err,"Reconcile NuodbInsightsServer failed: Failed reconcileESTemplates().",
			"Name", request.Name, "Namespace", request.Namespace)
		return result, err
	}

	result, err = reconcileESPipelines(request, utils.ESClusterName, esClient)
	if err != nil {
		log.Error(err,"Reconcile NuodbInsightsServer failed: Failed reconcileESPipelines().",
			"Name", request.Name, "Namespace", request.Namespace)
		return result, err
	}

	result, err = reconcileLogstashCluster(r, request, instance, utils.LogstashClusterName)
	if err != nil {
		log.Error(err,"Reconcile NuodbInsightsServer failed: Failed reconcileLogstashCluster().",
			"Name", request.Name, "Namespace", request.Namespace)
		return result, err
	}

	result, err = reconcileGrafanaCluster(r, request, instance,utils.GrafanaClusterName, rm)
	if err != nil {
		log.Error(err,"Reconcile NuodbInsightsServer failed: Failed reconcileGrafanaCluster().",
			"Name", request.Name, "Namespace", request.Namespace)
		return result, err
	}

	log.Info("Reconcile NuodbInsightsServer Successful.", "Name", request.Name,
		"Namespace", request.Namespace)
	return result, err
}

func createNuodbInsightsServerNuodbv2alpha1CRD(thisClient client.Client,
	crd *v1beta1.CustomResourceDefinition) error {
	log.Info("Create", "CRD", crd.Name)
	// We don't call controllerutil.SetControllerReference(instance, crd, thisScheme)
	// because it's ok to leave the CRDs installed.
	err := thisClient.Create(context.TODO(), crd)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func reconcileCustomResourceDefinition(thisClient client.Client,
	crd *v1beta1.CustomResourceDefinition)(reconcile.Result, error) {
	var err error = nil
	_, err = utils.GetCRD(crd.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = createNuodbInsightsServerNuodbv2alpha1CRD(thisClient, crd)
		}
	}
	return reconcile.Result{}, err
}

func reconcileNamespace(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	ns *corev1.Namespace)(reconcile.Result, error) {
	var err error = nil
	_, err = utils.GetNamespace(ns.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateNamespace(instance, thisClient, thisScheme, ns)
		}
	}
	return reconcile.Result{}, err
}

func reconcileSecret(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	secret *corev1.Secret, namespace string)(reconcile.Result, error) {
	if secret.Namespace == "" {
		secret.Namespace = namespace
	}
	var err error = nil
	_, err = utils.GetSecret(thisClient, secret.Namespace, secret.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateSecret(instance, thisClient, thisScheme, secret)
		}
	} else {
		err = thisClient.Update(context.TODO(), secret)
	}
	return reconcile.Result{}, err
}

func reconcileClusterRole(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	clusterRole *rbacv12.ClusterRole)(reconcile.Result, error) {
	var err error = nil
	_, err = utils.GetClusterRole(thisClient, clusterRole.Namespace, clusterRole.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateClusterRole(instance, thisClient, thisScheme, clusterRole)
		}
	} else {
		err = thisClient.Update(context.TODO(), clusterRole)
	}
	return reconcile.Result{}, err
}

func reconcileClusterRoleBinding(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	clusterRoleBinding *rbacv12.ClusterRoleBinding)(reconcile.Result, error) {
	var err error = nil
	_, err = utils.GetClusterRoleBinding(thisClient, clusterRoleBinding.Namespace, clusterRoleBinding.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateClusterRoleBinding(instance, thisClient, thisScheme, clusterRoleBinding)
		}
	} else {
		err = thisClient.Update(context.TODO(), clusterRoleBinding)
	}
	return reconcile.Result{}, err
}

func reconcileRole(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	role *rbacv12.Role, namespace string)(reconcile.Result, error) {
	var err error = nil
	if role.Namespace == "" {
		role.Namespace = namespace
	}
	_, err = utils.GetRole(thisClient, role.Namespace, role.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateRole(instance, thisClient, thisScheme, role)
		}
	} else {
		err = thisClient.Update(context.TODO(), role)
	}
	return reconcile.Result{}, err
}

func reconcileRoleBinding(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	roleBinding *rbacv12.RoleBinding, namespace string)(reconcile.Result, error) {
	var err error = nil
	if roleBinding.Namespace == "" {
		roleBinding.Namespace = namespace
	}
	_, err = utils.GetRoleBinding(thisClient, roleBinding.Namespace, roleBinding.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateRoleBinding(instance, thisClient, thisScheme, roleBinding)
		}
	} else {
		err = thisClient.Update(context.TODO(), roleBinding)
	}
	return reconcile.Result{}, err
}

func reconcileServiceAccount(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	serviceAccount *corev1.ServiceAccount, namespace string)(reconcile.Result, error) {
	var err error = nil
	if serviceAccount.Namespace == "" {
		serviceAccount.Namespace = namespace
	}
	_, err = utils.GetServiceAccount(thisClient, serviceAccount.Namespace, serviceAccount.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateServiceAccount(instance, thisClient, thisScheme, serviceAccount)
		}
	} else {
		err = thisClient.Update(context.TODO(), serviceAccount)
	}
	return reconcile.Result{}, err
}

func reconcileStatefulSet(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	statefulSet *appsv1.StatefulSet, namespace string)(reconcile.Result, error) {
	var err error = nil
	if statefulSet.Namespace == "" {
		statefulSet.Namespace = namespace
	}
	_, err = utils.GetStatefulSetV1(thisClient, statefulSet.Namespace, statefulSet.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateStatefulSetV1(instance, thisClient, thisScheme, statefulSet)
		}
	} else {
		err = thisClient.Update(context.TODO(), statefulSet)
	}
	return reconcile.Result{}, err
}

func reconcileDeployment(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	deployment *appsv1.Deployment, namespace string)(reconcile.Result, error) {
	var err error = nil
	if deployment.Namespace == "" {
		deployment.Namespace = namespace
	}
	_, err = utils.GetDeployment(thisClient, deployment.Namespace, deployment.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = utils.CreateDeployment(instance, thisClient, thisScheme, deployment)
		}
	} else {
		err = thisClient.Update(context.TODO(), deployment)
	}
	return reconcile.Result{}, err
}

func reconcileRuntimeObject(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer,
	runtimeObject runtime.Object, namespace string)(reconcile.Result, error) {
	var err error = nil
	kindObj := runtimeObject.GetObjectKind()
	thisKind := kindObj.GroupVersionKind().Kind
	switch thisKind {
		case "CustomResourceDefinition":
			ret, err := reconcileCustomResourceDefinition(thisClient, runtimeObject.(*v1beta1.CustomResourceDefinition))
			if err != nil {
				return ret, err
			}
		case "Namespace":
			ret, err := reconcileNamespace(thisClient, thisScheme, instance, runtimeObject.(*corev1.Namespace))
			if err != nil {
				return ret, err
			}
		case "Secret":
			ret, err := reconcileSecret(thisClient, thisScheme, instance, runtimeObject.(*corev1.Secret), namespace)
			if err != nil {
				return ret, err
			}
		case "Role":
			ret, err := reconcileRole(thisClient, thisScheme, instance, runtimeObject.(*rbacv12.Role), namespace)
			if err != nil {
				return ret, err
			}
		case "RoleBinding":
			ret, err := reconcileRoleBinding(thisClient, thisScheme, instance, runtimeObject.(*rbacv12.RoleBinding), namespace)
			if err != nil {
				return ret, err
			}
		case "ClusterRole":
			ret, err := reconcileClusterRole(thisClient, thisScheme, instance, runtimeObject.(*rbacv12.ClusterRole))
			if err != nil {
				return ret, err
			}
		case "ClusterRoleBinding":
			ret, err := reconcileClusterRoleBinding(thisClient, thisScheme, instance, runtimeObject.(*rbacv12.ClusterRoleBinding))
			if err != nil {
				return ret, err
			}
		case "ServiceAccount":
			ret, err := reconcileServiceAccount(thisClient, thisScheme, instance, runtimeObject.(*corev1.ServiceAccount), namespace)
			if err != nil {
				return ret, err
			}
		case "StatefulSet":
			ret, err := reconcileStatefulSet(thisClient, thisScheme, instance, runtimeObject.(*appsv1.StatefulSet), namespace)
			if err != nil {
				return ret, err
			}
		case "Deployment":
			ret, err := reconcileDeployment(thisClient, thisScheme, instance, runtimeObject.(*appsv1.Deployment), namespace)
			if err != nil {
				return ret, err
			}
		default:
			msg := fmt.Sprintf("Insights-Server invalid resource kind: %s", thisKind)
			err = apierrors.NewBadRequest(msg)
			log.Error(err, msg)
			return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

// Create Elastic Cloud on K8s
func createECKAllInOne(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer, allInOneYamlFilename, namespace string)(reconcile.Result, error) {
	runtimeObjects, err := utils.GetMultipleRuntimeObjectFromYamlFile(allInOneYamlFilename)
	if err != nil {
		log.Error(err, "Internal Error: unable to read ECK All In One Yaml file.")
		return reconcile.Result{}, err
	}

	// Process K8s Kinds in this order
	processKindOrder := [] string {
		"CustomResourceDefinition",
		"Namespace",
		"Secret",
		"ClusterRole",
		"ClusterRoleBinding",
		"ServiceAccount",
		"StatefulSet" }

	// First check that processKindOrder has all of the runtime Objects kinds.
	for _, obj := range runtimeObjects {
		kindObj := obj.GetObjectKind()
		thisKind := kindObj.GroupVersionKind().Kind
		foundKind := false
		for _, processKind := range processKindOrder {
			if thisKind == processKind {
				foundKind = true
				break
			}
		}
		if foundKind == false {
			msg := fmt.Sprintf("Unknown kind %s in function createECKAllInOne()", thisKind)
			err = errors.NewBadRequest(msg)
			log.Error(err, msg)
			return reconcile.Result{}, err
		}
	}

	// Process all of the runtimeObject in processKindOrder.
	for _, processKind := range processKindOrder {
		for _, obj := range runtimeObjects {
			kindObj := obj.GetObjectKind()
			thisKind := kindObj.GroupVersionKind().Kind
			if thisKind == processKind {
				ret, err := reconcileRuntimeObject(thisClient, thisScheme, instance, obj, namespace)
				if err != nil {
					return ret, err
				}
			}
		}
	}

	return reconcile.Result{}, err
}

// Reconcile Elastic Cloud on K8s
func reconcileECKAllInOne(thisClient client.Client, thisScheme *runtime.Scheme, instance *nuodbv2alpha1.NuodbInsightsServer, allInOneYamlFilename string, namespace string)(reconcile.Result, error) {
	log.Info("Reconcile ECKAllInOne")
	//_, err := utils.GetApiResources(utils.ElasticSearchGroupVersion)
	kc, err := utils.GetK8sClientSet()
	if err != nil {
		return reconcile.Result{}, err
	}
	_, err = kc.AppsV1().StatefulSets("elastic-system").Get("elastic-operator", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ECKAllInOne not found, creating...")
			result, err := createECKAllInOne(thisClient, thisScheme, instance, allInOneYamlFilename, namespace)
			if err != nil {
				log.Error(err, "Unable to create ECK All In One")
				return result, err
			}
			return result, err
		}
	}

	return reconcile.Result{}, nil
}

func getOwnerReference(r *ReconcileNuodbInsightsServer, namespacedName types.NamespacedName) (metav1.OwnerReference, error) {
	blockOwnerDeletionBool := true
	controllerBool := true
	ownRef := metav1.OwnerReference{}
	// Fetch the NuodbInsightsServer instance
	instance := &nuodbv2alpha1.NuodbInsightsServer{}
	err := r.client.Get(context.TODO(), namespacedName, instance)
	if err != nil {
		log.Error(err, "Unable to get NuodbInsightsServer instance in getOwnerReference()")
		return ownRef, err
	}
	ownRef.APIVersion = "nuodb.com/v2alpha1"
	ownRef.Kind = "NuodbInsightsServer"
	ownRef.Name = instance.Name
	ownRef.UID = instance.UID
	ownRef.BlockOwnerDeletion = &blockOwnerDeletionBool
	ownRef.Controller = &controllerBool
	return ownRef, err
}

func reconcileESTemplate(request reconcile.Request,
	esClusterName string, templateConfigFile string, esClient *elasticsearch.Client) (reconcile.Result, error) {
	log.Info("Reconcile ElasticSearch Template", "esClusterName", esClusterName,
		"request.Namespace", request.Namespace, "templateConfigFile", templateConfigFile)
	var err error = nil
	templateName := path.Base(templateConfigFile)
	templateSlice := []string{templateName}
	resp, err := esClient.Indices.ExistsTemplate(templateSlice)
	if err != nil {
		return reconcile.Result{}, err
	}
	if resp.StatusCode == 404 {
		var r io.Reader
		var err error
		r, err = os.Open(templateConfigFile)
		if err != nil {
			return reconcile.Result{}, err
		}
		log.Info("Creating ElasticSearch Template", "esClusterName", esClusterName,
			"request.Namespace", request.Namespace, "templateConfigFile", templateConfigFile)
		resp, err = esClient.Indices.PutTemplate(templateName, r)
		if err != nil {
			log.Error(err,"reconcileESTemplate failed to create Template",
				"esClusterName", esClusterName, "request.Namespace", request.Namespace,
				"templateConfigFile", templateConfigFile)
			return reconcile.Result{}, err
		}
		if resp.IsError() {
			msg := "reconcileESTemplate failed to create Template"
			err = apierrors.NewBadRequest(msg)
			log.Error(err, msg,"esClusterName", esClusterName, "request.Namespace", request.Namespace,
				"templateConfigFile", templateConfigFile)
			return reconcile.Result{}, err
		}
		log.Info("Successfully created ElasticSearch Template", "esClusterName", esClusterName,
			"request.Namespace", request.Namespace, "templateConfigFile", templateConfigFile)

	} else if resp.IsError() {
		msg := "reconcileESTemplate failed to locate Template"
		err = apierrors.NewBadRequest(msg)
		log.Error(err, msg, "esClusterName", esClusterName, "request.Namespace", request.Namespace,
			"templateConfigFile", templateConfigFile)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}

func reconcileESTemplates(request reconcile.Request, esClusterName string,
	esClient *elasticsearch.Client) (reconcile.Result, error) {
	log.Info("Reconcile ElasticSearch Templates", "esClusterName", esClusterName, "request.Namespace", request.Namespace)
	files, err := utils.GetESTemplateConfigFiles()
	if err != nil {
		return reconcile.Result{}, err
	}
	for i := range files {
		ret, err := reconcileESTemplate(request, esClusterName, files[i], esClient)
		if err != nil {
			return ret, err
		}
	}
	return reconcile.Result{}, err
}

func reconcileESPipeline(request reconcile.Request,
	esClusterName string, pipelineConfigFile string, esClient *elasticsearch.Client) (reconcile.Result, error) {
	log.Info("Reconcile ElasticSearch Pipeline", "esClusterName", esClusterName,
		"request.Namespace", request.Namespace, "pipelineConfigFile", pipelineConfigFile)
	pipelineName := path.Base(pipelineConfigFile)
	getPipelineFunc := esClient.Ingest.GetPipeline
	pipelineRequest := getPipelineFunc.WithPipelineID(pipelineName)
	resp, err := getPipelineFunc(pipelineRequest)
	if err != nil {
		return reconcile.Result{}, err
	}
	if resp.StatusCode == 404 {
		var r io.Reader
		var err error
		r, err = os.Open(pipelineConfigFile)
		if err != nil {
			return reconcile.Result{}, err
		}
		log.Info("Creating ElasticSearch Pipeline", "esClusterName", esClusterName,
			"request.Namespace", request.Namespace, "pipelineConfigFile", pipelineConfigFile)
		resp, err = esClient.Ingest.PutPipeline(pipelineName, r)
		if err != nil {
			log.Error(err,"reconcileESPipeline failed to create Pipeline",
				"esClusterName", esClusterName, "request.Namespace", request.Namespace,
				"pipelineConfigFile", pipelineConfigFile)
			return reconcile.Result{}, err
		}
		if resp.IsError() {
			msg := "reconcileESPipeline failed to create Pipeline"
			err = apierrors.NewBadRequest(msg)
			log.Error(err, msg,"esClusterName", esClusterName, "request.Namespace", request.Namespace,
				"pipelineConfigFile", pipelineConfigFile)
			return reconcile.Result{}, err
		}
		log.Info("Successfully created ElasticSearch Pipeline", "esClusterName", esClusterName,
			"request.Namespace", request.Namespace, "pipelineConfigFile", pipelineConfigFile)

	} else if resp.IsError() {
		msg := "reconcileESPipeline failed to locate Pipeline"
		err = apierrors.NewBadRequest(msg)
		log.Error(err, msg, "esClusterName", esClusterName, "request.Namespace", request.Namespace,
			"pipelineConfigFile", pipelineConfigFile)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, err
}


func reconcileESPipelines(request reconcile.Request, esClusterName string, esClient *elasticsearch.Client) (reconcile.Result, error) {
	log.Info("Reconcile ElasticSearch Pipelines", "esClusterName", esClusterName, "request.Namespace", request.Namespace)
	files, err := utils.GetESPipelineConfigFiles()
	if err != nil {
		return reconcile.Result{}, err
	}
	for i := range files {
		ret, err := reconcileESPipeline(request, esClusterName, files[i], esClient)
		if err != nil {
			return ret, err
		}
	}
	return reconcile.Result{}, err
}

// Reconcile Elasticsearch Cluster on K8s
func reconcileESCluster(r *ReconcileNuodbInsightsServer, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer, esClusterName string, rm meta.RESTMapper) (reconcile.Result, error) {
	msg := fmt.Sprintf("Reconcile %s in namespace: %s", esClusterName, request.Namespace )
	log.Info(msg)
	_, err := utils.GetApiResources(utils.ElasticSearchGroupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "ECKAllInOne not found.")
			return reconcile.Result{}, err
		}
	}
	_, err = utils.GetElasticsearchCR(esClusterName, request.Namespace, rm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("Creating %s in namespace: %s", esClusterName, request.Namespace)
			log.Info(msg)
			nodeConfig := commonv1alpha1.Config {
			Data: map[string]interface{}{
				"node.master": true,
				"node.data": true,
				"node.ingest": true,
				},
			}
			esCR := esv1alpha1.Elasticsearch {
				TypeMeta: metav1.TypeMeta { APIVersion:"elasticsearch.k8s.elastic.co/v1alpha1", Kind:"Elasticsearch"},
				ObjectMeta: metav1.ObjectMeta { Name:"insights-escluster", Namespace:request.Namespace},
				Spec: esv1alpha1.ElasticsearchSpec{
					Version: "7.3.0",
					HTTP: commonv1alpha1.HTTPConfig{
						Service: commonv1alpha1.ServiceTemplate{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer} },
						TLS:     commonv1alpha1.TLSOptions{},
					},
					Nodes: [] esv1alpha1.NodeSpec{
						{
							Config: &nodeConfig,
							VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
								{
									ObjectMeta: metav1.ObjectMeta{
										Name: "elasticsearch-data",
									},
									Spec: corev1.PersistentVolumeClaimSpec{
										AccessModes: []corev1.PersistentVolumeAccessMode{
											corev1.ReadWriteOnce,
										},
										Resources: corev1.ResourceRequirements {
											Requests: corev1.ResourceList{
												corev1.ResourceStorage: resource.MustParse("2Gi"),
											},
										},
									},
								},
							},
							NodeCount: 1,
						},
					},
				},
				Status: esv1alpha1.ElasticsearchStatus {},
			}
			if instance.Spec.StorageClass != "" {
				esCR.Spec.Nodes[0].VolumeClaimTemplates[0].Spec.StorageClassName = &instance.Spec.StorageClass
			}
			ownRef, err := getOwnerReference(r, request.NamespacedName)
			if err != nil {
				return reconcile.Result{}, trace.Wrap(err)
			}
			ownRefs := []metav1.OwnerReference{ownRef}
			esCR.SetOwnerReferences(ownRefs)
			err = utils.CreateElasticsearch(r.client, &esCR)
			if err != nil {
				msg := fmt.Sprintf("Unable to create CR %s.", esClusterName)
				log.Error(err, msg)
				return reconcile.Result{}, trace.Wrap(err)
			}
			statusCount := 0
			es, err := utils.GetElasticsearch(r.client, request.Namespace, utils.ESClusterName)
			if err != nil || es == nil {
				return reconcile.Result{}, err
			}
			for es.Status.Phase != v1alpha1.ElasticsearchOperationalPhase {
				statusCount++
				log.Info("Waiting for ES Cluster to become operational",
					"esClusterName" ,esClusterName, "es.Status.Phase", es.Status.Phase )
				time.Sleep(time.Second * 5)
				if statusCount > 20 {
					msg := fmt.Sprintf("Timeout waiting for ES Cluster: %s.", esClusterName)
					err := errors.NewTimeoutError(msg, 120)
					log.Error(err, "esClusterName", esClusterName)
					return reconcile.Result{}, err
				}
				es, err = utils.GetElasticsearch(r.client, request.Namespace, utils.ESClusterName)
				if err != nil || es == nil {
					return reconcile.Result{}, err
				}
			}
		} else {
			return reconcile.Result{}, trace.Wrap(err)
		}
		err = nil
	} else {
		es, err := utils.GetElasticsearch(r.client, request.Namespace, utils.ESClusterName)
		if err != nil {
			msg := fmt.Sprintf("Unable to get elasticsearch %s in namespace: %s",
				utils.ESClusterName, request.Namespace)
			log.Error(err, msg)
			return reconcile.Result{}, err
		}
		updateFlag := false
		currentNodeCount:= es.Spec.Nodes[0].NodeCount
		desiredNodeCount := instance.Spec.ElasticNodeCount
		if currentNodeCount != desiredNodeCount {
			updateFlag = true
			es.Spec.Nodes[0].NodeCount = desiredNodeCount
			msg := fmt.Sprintf("Updating nodecount to %d on elasticsearch %s in namespace: %s",
				desiredNodeCount, utils.ESClusterName, request.Namespace)
			log.Info(msg)
		}
		currentESVersion := es.Spec.Version
		desiredESVersion := instance.Spec.ElasticVersion
		if currentESVersion != desiredESVersion {
			updateFlag = true
			es.Spec.Version = desiredESVersion
		}
		if updateFlag {
			err = r.client.Update(context.TODO(), es)
			if err != nil {
				msg := fmt.Sprintf("Unable to update elasticsearch %s in namespace: %s",
					utils.ESClusterName, request.Namespace)
				log.Error(err, msg)
				return reconcile.Result{}, err

			}
		}
		err = nil
	}
	return reconcile.Result{}, err
}

func deleteESCluster(thisClient client.Client, namespace string, rm meta.RESTMapper) error {
	tryCount := 1
	for {
		msg := fmt.Sprintf("Deleting %s in namespace: %s", utils.ESClusterName, namespace)
		log.Info(msg)
		cr, err := utils.GetElasticsearchCR(utils.ESClusterName, namespace, rm)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			msg := fmt.Sprintf("Unable to find  %s in namespace: %s", utils.ESClusterName, namespace)
			log.Info(msg)
		} else if cr != nil {
			err = thisClient.Delete(context.TODO(), cr)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				msg := fmt.Sprintf("Unable to delete  %s in namespace: %s", utils.ESClusterName, namespace)
				log.Error(err, msg)
			}
		}
		if tryCount > 5 {
			msg := "too many retries in deleteESCluster"
			err = apierrors.NewTimeoutError(msg, 5)
			log.Error(err, msg, "namespace", namespace, "name", utils.ESClusterName)
		}
		tryCount++
		sleepDuration := time.Duration(tryCount) * time.Second
		time.Sleep(sleepDuration)
	}
}

func (r *ReconcileNuodbInsightsServer) deleteExternalResources(instance *nuodbv2alpha1.NuodbInsightsServer, request reconcile.Request, rm meta.RESTMapper) error {
	err := deleteGrafanaCluster(r.client, request.Namespace, rm)
	if err != nil {
		return err
	}

	err = deleteLogstashCluster(request.Namespace)
	if err != nil {
		return err
	}

	err = deleteKibanaCluster(r.client, request.Namespace, rm)
	if err != nil {
		return err
	}

	err = deleteESCluster(r.client, request.Namespace, rm)
	if err != nil {
		return err
	}

	return nil
}

// Reconcile Kibana Cluster on K8s
func reconcileKibanaCluster(r *ReconcileNuodbInsightsServer, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer, kibanaClusterYamlFile string,  kibanaClusterName string, rm meta.RESTMapper) (reconcile.Result, error) {
	msg := fmt.Sprintf("Reconcile %s in namespace: %s", kibanaClusterName, request.Namespace )
	log.Info(msg)
	_, err := utils.GetApiResources(utils.KibanaGroupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Kibana API not found.")
			return reconcile.Result{}, err
		}
	}
	_, err = utils.GetKibanaCR(kibanaClusterName, request.Namespace, rm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("Creating %s in namespace: %s", kibanaClusterName, request.Namespace)
			log.Info(msg)
			runtimeObj, err := utils.GetSingleRuntimeObjectFromYamlFile(kibanaClusterYamlFile)
			if err != nil {
				return reconcile.Result{}, err
			}
			ownRef, err := getOwnerReference(r, request.NamespacedName)
			if err != nil {
				return reconcile.Result{}, trace.Wrap(err)
			}
			ownRefs := []metav1.OwnerReference{ownRef}
			_, err = utils.CreateKibanaCR(runtimeObj, request.Namespace, ownRefs, rm)
			if err != nil {
				msg := fmt.Sprintf("Unable to create CR %s.", kibanaClusterName)
				log.Error(err, msg)
				return reconcile.Result{}, trace.Wrap(err)
			}
		} else {
			return reconcile.Result{}, trace.Wrap(err)
		}
		err = nil
	} else {
		kibana, err := utils.GetKibana(r.client, request.Namespace, utils.KibanaClusterName)
		if err != nil {
			msg := fmt.Sprintf("Unable to get Kibana %s in namespace: %s",
				utils.KibanaClusterName, request.Namespace)
			log.Error(err, msg)
			return reconcile.Result{}, err
		}
		updateFlag := false
		currentNodeCount:= kibana.Spec.NodeCount
		desiredNodeCount := instance.Spec.KibanaNodeCount
		if currentNodeCount != desiredNodeCount {
			updateFlag = true
			kibana.Spec.NodeCount = desiredNodeCount
			msg := fmt.Sprintf("Updating nodecount to %d on kibana %s in namespace: %s",
				desiredNodeCount, utils.KibanaClusterName, request.Namespace)
			log.Info(msg)
		}
		currentESVersion := kibana.Spec.Version
		desiredESVersion := instance.Spec.KibanaVersion
		if currentESVersion != desiredESVersion {
			updateFlag = true
			kibana.Spec.Version = desiredESVersion
		}
		if updateFlag {
			err = r.client.Update(context.TODO(), kibana)
			if err != nil {
				msg := fmt.Sprintf("Unable to update Kibana %s in namespace: %s",
					utils.GrafanaClusterName, request.Namespace)
				log.Error(err, msg)
				return reconcile.Result{}, err

			}
		}
	}
	return reconcile.Result{}, err
}

func deleteKibanaCluster(thisClient client.Client, namespace string, rm meta.RESTMapper) error {
	msg := fmt.Sprintf("Deleting %s in namespace: %s", utils.KibanaClusterName, namespace )
	log.Info(msg)
	cr, err := utils.GetKibanaCR(utils.KibanaClusterName, namespace, rm)
	if err != nil {
		msg := fmt.Sprintf("Unable to find  %s in namespace: %s", utils.KibanaClusterName, namespace )
		log.Info(msg)
		return nil
	}
	err = thisClient.Delete(context.TODO(), cr)
	if err != nil {
		msg := fmt.Sprintf("Unable to delete  %s in namespace: %s", utils.KibanaClusterName, namespace )
		log.Error(err, msg)
		return err
	}
	return nil
}

// Reconcile Grafana Cluster on K8s
func reconcileGrafanaCR(r *ReconcileNuodbInsightsServer, request reconcile.Request,
	grafanaClusterYamlFile string, grafanaClusterName string, rm meta.RESTMapper) (reconcile.Result, error) {
	msg := fmt.Sprintf("Reconcile %s in namespace: %s", grafanaClusterName, request.Namespace )
	log.Info(msg)
	_, err := utils.GetApiResources(utils.GrafanaGroupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Grafana API not found.")
			return reconcile.Result{}, err
		}
	}
	_, err = utils.GetGrafanaCR(grafanaClusterName, request.Namespace, rm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("Creating %s in namespace: %s", grafanaClusterName, request.Namespace)
			log.Info(msg)
			runtimeObj, err := utils.GetSingleRuntimeObjectFromYamlFile(grafanaClusterYamlFile)
			if err != nil {
				return reconcile.Result{}, err
			}
			ownRef, err := getOwnerReference(r, request.NamespacedName)
			if err != nil {
				return reconcile.Result{}, trace.Wrap(err)
			}
			ownRefs := []metav1.OwnerReference{ownRef}
			runtimeObj.(*grafanav1alpha1.Grafana).Namespace = request.Namespace
			_, err = utils.CreateGrafanaCR(runtimeObj, request.Namespace, ownRefs, rm)
			if err != nil {
				msg := fmt.Sprintf("Unable to create CR %s.", grafanaClusterName)
				log.Error(err, msg)
				return reconcile.Result{}, trace.Wrap(err)
			}
		} else {
			return reconcile.Result{}, trace.Wrap(err)
		}
		err = nil
	} else {
		grafana, err := utils.GetGrafana(r.client, request.Namespace, utils.GrafanaClusterName)
		if err != nil {
			msg := fmt.Sprintf("Unable to get Grafana %s in namespace: %s",
				utils.GrafanaClusterName, request.Namespace)
			log.Error(err, msg)
			return reconcile.Result{}, err
		}
		updateFlag := false
		// TODO
		if updateFlag {
			err = r.client.Update(context.TODO(), grafana)
			if err != nil {
				msg := fmt.Sprintf("Unable to update Kibana %s in namespace: %s",
					utils.GrafanaClusterName, request.Namespace)
				log.Error(err, msg)
				return reconcile.Result{}, err

			}
		}
	}
	return reconcile.Result{}, err
}

// Reconcile Grafana DataSource on K8s
func reconcileGrafanaDataSourceCR(r *ReconcileNuodbInsightsServer, request reconcile.Request,
	grafanaClusterName string, rm meta.RESTMapper) (reconcile.Result, error) {
	msg := fmt.Sprintf("Reconcile %s data source %s in namespace: %s", grafanaClusterName,
		utils.GrafanaClusterDataSourceName, request.Namespace )
	log.Info(msg)
	_, err := utils.GetApiResources(utils.GrafanaGroupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Grafana API not found.")
			return reconcile.Result{}, err
		}
	}
	_, err = utils.GetGrafanaDataSourceCR(utils.GrafanaClusterDataSourceName, request.Namespace, rm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("Creating %s data source %s in namespace: %s", grafanaClusterName,
				utils.GrafanaClusterDataSourceName, request.Namespace)
			log.Info(msg)
			dsJsonData := grafanav1alpha1.GrafanaDataSourceJsonData{TlsAuth: true, TlsSkipVerify:true,
				TimeInterval:"10s", EsVersion:70, TimeField:"timestamp", Interval:"Weekly"}
			dsSecureJsonData := grafanav1alpha1.GrafanaDataSourceSecureJsonData{}
			esPassword, err := utils.GetESPassword(request.Namespace)
			if err != nil {
				return reconcile.Result{}, err
			}
			dsSecureJsonData.BasicAuthPassword = esPassword
			esCerts, err := utils.GetSecret(r.client, request.Namespace, utils.ESClusterHttpCertsInternal)
			if err != nil {
				log.Error(err, "Unable to locate.",
					"secret", utils.ESClusterHttpCertsInternal,
					"namespace", request.Namespace)
				return reconcile.Result{}, err
			}
			tlsClientCertData, ok := esCerts.Data["tls.crt"]
			if !ok {
				msg = "Unable to locate 'tls.crt'."
				err := errors.NewBadRequest(msg)
				log.Error(err, msg,
					"secret", utils.ESClusterHttpCertsInternal,
					"namespace", request.Namespace)
				return reconcile.Result{}, err
			}
			dsSecureJsonData.TlsClientCert = string(tlsClientCertData)
			tlsClientKeyData, ok := esCerts.Data["tls.key"]
			if !ok {
				msg = "Unable to locate 'tls.key'."
				err := errors.NewBadRequest(msg)
				log.Error(err, msg,
					"secret", utils.ESClusterHttpCertsInternal,
					"namespace", request.Namespace)
				return reconcile.Result{}, err
			}
			dsSecureJsonData.TlsClientKey = string(tlsClientKeyData)
			esService, err := utils.GetService(r.client, request.Namespace, utils.ESClusterService)
			if err != nil {
				log.Error(err, "Unable to locate.",
					"service", utils.ESClusterService,
					"namespace", request.Namespace)
				return reconcile.Result{}, err

			}
			dsFields := grafanav1alpha1.GrafanaDataSourceFields{Name: "Insights NuoMon", Type:"elasticsearch",
				Database:"ic_*", Access:"proxy", IsDefault:true, Version:1, Editable:true, BasicAuth:true}
			dsFields.JsonData = dsJsonData
			dsFields.SecureJsonData = dsSecureJsonData
			dsFields.Url = "https://" + esService.Spec.ClusterIP + ":9200"
			dsFields.BasicAuthUser = "elastic"
			dsSpec := grafanav1alpha1.GrafanaDataSourceSpec{Name: "insights-es-middleware.yaml" }
			dsSpec.Datasources = append(dsSpec.Datasources, dsFields)
			dscr := grafanav1alpha1.GrafanaDataSource{}
			dscr.Namespace = request.Namespace
			dscr.Name = utils.GrafanaClusterDataSourceName
			dscr.Spec = dsSpec
			dscr.Labels = map[string]string{}
			dscr.Labels["nuodb-insights-server"] = request.Name

			gvk := schema.GroupVersionKind{
				Group: utils.GrafanaGroup,
				Version: utils.GrafanaVersion,
				Kind: grafanav1alpha1.GrafanaDataSourceKind }
			dscr.SetGroupVersionKind(gvk)

			ownRef, err := getOwnerReference(r, request.NamespacedName)
			if err != nil {
				return reconcile.Result{}, trace.Wrap(err)
			}
			ownRefs := []metav1.OwnerReference{ownRef}

			_, err = utils.CreateGrafanaDataSourceCR(&dscr, request.Namespace, ownRefs, rm)
			if err != nil {
				msg := fmt.Sprintf("Unable to create CR %s.", grafanaClusterName)
				log.Error(err, msg)
				return reconcile.Result{}, trace.Wrap(err)
			}
		} else {
			return reconcile.Result{}, trace.Wrap(err)
		}
		err = nil
	} else {

		grafanaDataSource, err := utils.GetGrafanaDataSourceCR(utils.GrafanaClusterDataSourceName, request.Namespace, rm)
		if err != nil {
			msg := fmt.Sprintf("Unable to get Grafana Data Source %s in namespace: %s",
				request.Name, request.Namespace)
			log.Error(err, msg)
			return reconcile.Result{}, err
		}
		updateFlag := false
		// TODO
		if updateFlag {
			err = r.client.Update(context.TODO(), grafanaDataSource)
			if err != nil {
				msg := fmt.Sprintf("Unable to update Grafana Data Source %s in namespace: %s",
					request.Name, request.Namespace)
				log.Error(err, msg)
				return reconcile.Result{}, err
			}
		}
	}
	return reconcile.Result{}, err
}

// Reconcile Grafana Dashboard on K8s
func reconcileGrafanaDashboardCR(r *ReconcileNuodbInsightsServer, request reconcile.Request,
	grafanaClusterName string, dashboardJsonFileName string, dashboardName string, rm meta.RESTMapper) (reconcile.Result, error) {
	msg := fmt.Sprintf("Reconcile %s dashboard in namespace: %s", grafanaClusterName, request.Namespace )
	log.Info(msg, "dashboardName", dashboardName, "dashboardJsonFileName", dashboardJsonFileName)
	_, err := utils.GetApiResources(utils.GrafanaGroupVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "Grafana API not found.")
			return reconcile.Result{}, err
		} else {
			log.Error(err, "Unable to obtain Grafana API.")
			return reconcile.Result{}, err
		}
	}

	cr, err := utils.GetGrafanaDashboardCR(dashboardName, request.Namespace, rm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("Creating %s dashboard %s in namespace: %s", grafanaClusterName,
				dashboardName, request.Namespace)
			log.Info(msg, "dashboardName", dashboardName, "dashboardJsonFileName", dashboardJsonFileName)

			jsonFile, err := ioutil.ReadFile(dashboardJsonFileName)
			if err != nil {
				log.Error(err, "Unable to read Grafana Dashboard file.",
					"dashboardJsonFileName", dashboardJsonFileName)
				return reconcile.Result{}, err
			}

			dashBoardSpec := grafanav1alpha1.GrafanaDashboardSpec{}
			dashBoardSpec.Name = dashboardName + "-dashboard.json"
			dashBoardSpec.Json = string(jsonFile)
			dashBoard := grafanav1alpha1.GrafanaDashboard{}
			dashBoard.Name = dashboardName
			dashBoard.Labels = map[string]string{}
			dashBoard.Labels["app"] = "grafana"
			dashBoard.Labels["nuodb-insights-server"] = request.Name
			dashBoard.Spec = dashBoardSpec

			gvk := schema.GroupVersionKind{
				Group: utils.GrafanaGroup,
				Version: utils.GrafanaVersion,
				Kind: grafanav1alpha1.GrafanaDashboardKind }
			dashBoard.SetGroupVersionKind(gvk)

			ownRef, err := getOwnerReference(r, request.NamespacedName)
			if err != nil {
				log.Error(err, "Unable to get owner reference.")
				return reconcile.Result{}, trace.Wrap(err)
			}
			ownRefs := []metav1.OwnerReference{ownRef}
			_, err = utils.CreateGrafanaDashboardCR(&dashBoard, request.Namespace, ownRefs, rm)
			if err != nil {
				msg := fmt.Sprintf("Unable to create CR %s.", dashboardName)
				log.Error(err, msg)
				return reconcile.Result{}, trace.Wrap(err)
			}
		} else {
			msg = fmt.Sprintf("Unable to create CR %s.", dashboardName)
			log.Error(err, msg)
			return reconcile.Result{}, trace.Wrap(err)
		}
		err = nil
	} else {
		msg := fmt.Sprintf("Nothing to do for %s dashboard in namespace: %s", cr.ObjectMeta.Name, request.Namespace )
		log.Info(msg, "dashboardName", dashboardName, "dashboardJsonFileName", dashboardJsonFileName)
		return reconcile.Result{}, nil
	}
	return reconcile.Result{}, err
}


type LogstashResource struct {
	name               string
	kind               string
	template           string
	templateFilename   string
	templateDecodedMap map[string]interface{}
	templateMetadata   map[string]interface{}
}

type LogstashResources struct {
	values               map[string]string
	logstashResourceList []LogstashResource
}

func processLogstashResources(logstashResources *LogstashResources) error {
	for templateFilename, template := range (*logstashResources).values {
		if template == "\n" {
			continue  // skip empty templates
		}
		if templateFilename == "logstash/templates/servicemonitor.yaml" {
			continue  // skip the servicemonitor, which we don't use, and it's K8s type may not be installed.
		}
		var logstashResource LogstashResource
		logstashResource.template = template
		logstashResource.templateFilename = templateFilename
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
		logstashResource.templateDecodedMap = m2

		if val, ok := m2["kind"]; ok {
			logstashResource.kind = val.(string)
			var metadata map[string]interface{}
			if err := mapstructure.Decode(m2["metadata"], &metadata); err != nil {
				log.Error(err, "Error mapstructure.Decode().")
				return utils.ConvertError(err)
			}
			logstashResource.name = metadata["name"].(string)
			logstashResource.templateMetadata = metadata
			(*logstashResources).logstashResourceList = append((*logstashResources).logstashResourceList, logstashResource)
		} else {
			log.Info("Skipping template that we didn't understand.", "template file:", templateFilename)
		}
	}
	return nil
}

func processLogstashTemplates(chartDir string, spec nuodbv2alpha1.NuodbInsightsServerSpec) (LogstashResources, error) {
	var logstashResources LogstashResources
	c, err := chartutil.Load(chartDir)
	if err != nil {
		log.Error(err,"Failed to process chart directory.")
		return logstashResources, err
	}

	options := chartutil.ReleaseOptions{Name: "insights-server-release", Time: timeconv.Now(), Namespace: "nuodb"}
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
		return logstashResources, err
	}
	for k, v := range mNew {
		str := fmt.Sprintf("%s", v.Value)
		cvals[k] = str
	}

	// Dynamically set the Logstash ElasticSearch settings based on current state.
	esClusterIP, err := utils.GetESClusterIP("nuodb") // TODO: fix hard coded namespace
	if err != nil {
		return logstashResources, err
	}
	newEsHost := "elasticsearch:\n host: " + esClusterIP + "\n port: 9200"
	newVals := chartutil.FromYaml(newEsHost)
	cvals.MergeInto(newVals)
	enableXpack := "config:\n  xpack.monitoring.collection.enabled: \"true\""
	newVals = chartutil.FromYaml(enableXpack)
	cvals.MergeInto(newVals)

	// convert our values back into config
	yvals, err := cvals.YAML()
	if err != nil {
		log.Error(err,"Failed to convert our values back into config.")
		return logstashResources, err
	}
	cc := &cpb.Config{Raw: yvals}
	valuesToRender, err := chartutil.ToRenderValuesCaps(c, cc, options, caps)
	if err != nil {
		log.Error(err,"Failed chartutil.ToRenderValuesCaps().")
		return logstashResources, err
	}
	if spec.StorageClass != "" {
		x, err := valuesToRender.Table("Values.persistence")
		if err != nil {
			log.Error(err,"Failed to get Values table.")
			return logstashResources, err
		}
		x["storageClass"] = spec.StorageClass
	}
	e := engine.New()


	out, err := e.Render(c, valuesToRender)
	if err != nil {
		log.Error(err,"Failed to render templates.")
		return logstashResources, err
	}
	logstashResources.values = out
	return logstashResources, err
}

func logstashResourcesInit(instance *nuodbv2alpha1.NuodbInsightsServer) (LogstashResources, error) {
	var logstashResources LogstashResources
	var err error = nil
	chartDir := utils.LogstashChartDir
	// TODO: when running in a docker container the base chartDir should be "/usr/local/etc/nuodb-operator/charts"
	logstashResources, err = processLogstashTemplates(chartDir, instance.Spec)
	if err != nil {
		log.Error(err, "Failed to process templates")
		return logstashResources, utils.ConvertError(err)
	}
	logstashResources.logstashResourceList = make([]LogstashResource, 0)
	err = processLogstashResources(&logstashResources)
	return logstashResources, err
}

func getnuodbv2alpha1NuodbInsightsServerInstance(r *ReconcileNuodbInsightsServer, request reconcile.Request) (*nuodbv2alpha1.NuodbInsightsServer, error) {
	// Fetch the Nuodb instance
	nuodbv2alpha1NuodbinsightsserverInstance := &nuodbv2alpha1.NuodbInsightsServer{}
	err := r.client.Get(context.TODO(), request.NamespacedName, nuodbv2alpha1NuodbinsightsserverInstance)
	return nuodbv2alpha1NuodbinsightsserverInstance, err
}

func createLogstashPodDisruptionBudget(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource) (*pv1b1.PodDisruptionBudget, error) {
	podDisruptionBudget, err := utils.CreatePodDisruptionBudgetV1b1FromTemplate(instance, thisClient, thisScheme, logstashResource.template, request.Namespace)
	return podDisruptionBudget, err
}

func createLogstashService(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource) (*corev1.Service, error) {
	service, err := utils.CreateServiceFromTemplate(instance, thisClient, thisScheme, logstashResource.template, request.Namespace)
	return service, err
}

func createLogstashServiceAccount(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource) (*corev1.ServiceAccount, error) {
	serviceAccount, err := utils.CreateServiceAccountFromTemplate(instance, thisClient, thisScheme, logstashResource.template, request.Namespace)
	return serviceAccount, err
}

func createLogstashConfigMap(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource) (*corev1.ConfigMap, error) {
	configMap, err := utils.CreateConfigMapFromTemplate(instance, thisClient, thisScheme, logstashResource.template, request.Namespace)
	return configMap, err
}

func createLogstashIngress(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource) (*extensions.Ingress, error) {
	var ingress *extensions.Ingress = nil
	ingress, err := utils.DecodeIngressTemplate(logstashResource.template)
	if err != nil {
		return ingress, err
	}
	ingress.Namespace = request.Namespace
	if err := controllerutil.SetControllerReference(instance, ingress, thisScheme); err != nil {
		return ingress, trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), ingress)
	if err != nil {
		return ingress, trace.Wrap(err)
	}
	return ingress, err
}

func createLogstashStatefulSetV1(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource) (*appsv1.StatefulSet, error) {
	statefulSet, err := utils.CreateStatefulSetV1FromTemplate(instance, thisClient, thisScheme, logstashResource.template, request.Namespace)
	return statefulSet, err
}

func reconcileLogstashPodDisruptionBudget(r *ReconcileNuodbInsightsServer, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource) (*pv1b1.PodDisruptionBudget, reconcile.Result, error) {
	var podDisruptionBudget *pv1b1.PodDisruptionBudget = nil
	err := policy.AddToScheme(r.scheme)
	if err != nil {
		return podDisruptionBudget, reconcile.Result{}, err
	}
	config, err := utils.GetDefaultKubeConfig()
	if err != nil {
		return podDisruptionBudget, reconcile.Result{}, err
	}
	pc, err := ptv1beta1.NewForConfig(config)
	if err != nil {
		return podDisruptionBudget, reconcile.Result{}, err
	}
	pdbs := pc.PodDisruptionBudgets(request.Namespace)
	podDisruptionBudget, err = pdbs.Get(logstashResource.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			podDisruptionBudget, err = createLogstashPodDisruptionBudget(r.client, r.scheme, request, instance, logstashResource)
			if err != nil {
				return podDisruptionBudget, reconcile.Result{}, err
			}
		} else {
			return podDisruptionBudget, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return podDisruptionBudget, reconcile.Result{}, err
}

func reconcileLogstashService(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	nuoResource LogstashResource, namespace string) (*corev1.Service, reconcile.Result, error) {
	var service *corev1.Service = nil
	service, err := utils.GetService(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			service, err = createLogstashService(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return service, reconcile.Result{}, err
			}
		} else {
			return service, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return service, reconcile.Result{}, err
}

func reconcileLogstashServiceAccount(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	nuoResource LogstashResource, namespace string) (*corev1.ServiceAccount, reconcile.Result, error) {
	var serviceAccount *corev1.ServiceAccount = nil
	serviceAccount, err := utils.GetServiceAccount(thisClient, namespace, nuoResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			serviceAccount, err = createLogstashServiceAccount(thisClient, thisScheme, request, instance, nuoResource)
			if err != nil {
				return serviceAccount, reconcile.Result{}, err
			}
		} else {
			return serviceAccount, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return serviceAccount, reconcile.Result{}, err
}

func reconcileLogstashConfigMap(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource, namespace string) (*corev1.ConfigMap, reconcile.Result, error) {
	var configMap *corev1.ConfigMap = nil
	configMap, err := utils.GetConfigMap(thisClient, namespace, logstashResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			configMap, err = createLogstashConfigMap(thisClient, thisScheme, request, instance, logstashResource)
			if err != nil {
				return configMap, reconcile.Result{}, err
			}
		} else {
			return configMap, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return configMap, reconcile.Result{}, err
}

func reconcileLogstashIngress(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource, namespace string) (*extensions.Ingress, reconcile.Result, error) {
	var ingress *extensions.Ingress = nil
	ingress, err := utils.GetIngress(thisClient, namespace, logstashResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ingress, err = createLogstashIngress(thisClient, thisScheme, request, instance, logstashResource)
			if err != nil {
				return ingress, reconcile.Result{}, err
			}
		} else {
			return ingress, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	}
	return ingress, reconcile.Result{}, err
}

func reconcileLogstashStatefulSetV1(thisClient client.Client, thisScheme *runtime.Scheme, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer,
	logstashResource LogstashResource, namespace string) (*appsv1.StatefulSet, reconcile.Result, error) {
	var statefulSet *appsv1.StatefulSet = nil
	statefulSet, err := utils.GetStatefulSetV1(thisClient, namespace, logstashResource.name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			statefulSet, err = createLogstashStatefulSetV1(thisClient, thisScheme, request, instance, logstashResource)
			if err != nil {
				return statefulSet, reconcile.Result{}, err
			}
		} else {
			return statefulSet, reconcile.Result{Requeue: true, RequeueAfter: utils.ReconcileRequeueAfterDefault}, err
		}
	} else {

	}
	return statefulSet, reconcile.Result{}, err
}

// Reconcile Logstash
func reconcileLogstashCluster(r *ReconcileNuodbInsightsServer, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer, logstashClusterName string) (reconcile.Result, error) {
	msg := fmt.Sprintf("Reconcile %s in namespace: %s", logstashClusterName, request.Namespace )
	log.Info(msg)
	// Fetch the Nuodb instance
	instance, err := getnuodbv2alpha1NuodbInsightsServerInstance(r, request)
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

	logstashResources, err := logstashResourcesInit(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	processOrder := [] string {
		"ConfigMap",
		"PodDisruptionBudget",
		"ServiceAccount",
		"Service",
		"Ingress",
		"StatefulSet" }

	currentTime := time.Now()
	log.Info("Starting Reconcile request: " + currentTime.String())
	var rr reconcile.Result

	for item := range processOrder {
		for _, nuoResource := range logstashResources.logstashResourceList {
			if nuoResource.kind == processOrder[item] {
				switch nuoResource.kind {
				case "PodDisruptionBudget":
					_, rr, err := reconcileLogstashPodDisruptionBudget(r, request, instance, nuoResource)
					if err != nil || rr.Requeue  {
						return rr, err
					}
				case "ServiceAccount":
					_, rr, err = reconcileLogstashServiceAccount(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue  {
						return rr, err
					}
				case "Service":
					_, rr, err = reconcileLogstashService(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue  {
						return rr, err
					}
				case "ConfigMap":
					_, rr, err = reconcileLogstashConfigMap(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue  {
						return rr, err
					}
				case "Ingress":
					_, rr, err = reconcileLogstashIngress(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue  {
						return rr, err
					}
				case "StatefulSet":
					_, rr, err = reconcileLogstashStatefulSetV1(r.client, r.scheme, request, instance, nuoResource, request.Namespace)
					if err != nil || rr.Requeue  {
						return rr, err
					}
				}
			}
		}
	}

	namespace, err := utils.GetNamespace(request.Namespace)
	if err != nil {
		return reconcile.Result{}, nil
	}
	var namespaces corev1.NamespaceList
	namespaces.Items = append(namespaces.Items, *namespace)
	err = utils.AddLogstashServiceAccountsToSCC(&namespaces, "privileged")
	if err != nil {
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, err
}

func deleteLogstashCluster(namespace string) error {
	msg := fmt.Sprintf("Deleting %s in namespace: %s", utils.LogstashClusterName, namespace )
	log.Info(msg)
	// TODO: Anything to do here?
	return nil
}

func reconcileSingleObjectFromYamlFile(r *ReconcileNuodbInsightsServer, request reconcile.Request,
	instance *nuodbv2alpha1.NuodbInsightsServer, yamlFilename string) (reconcile.Result, error) {
	log.Info("reconcileSingleObjectFromYamlFile", "yamlFilename", yamlFilename, "namespace", request.Namespace)
	runtimeObject, err := utils.GetSingleRuntimeObjectFromYamlFile(yamlFilename)
	if err != nil {
		log.Error(err, "Internal Error: unable to process Yaml file.",
			"yamlFilename", yamlFilename)
		return reconcile.Result{}, err
	}
	ret, err := reconcileRuntimeObject(r.client, r.scheme, instance, runtimeObject, request.Namespace)
	if err != nil {
		return ret, err
	}
	return reconcile.Result{}, err
}

// Reconcile Grafana Cluster
func reconcileGrafanaCluster(r *ReconcileNuodbInsightsServer, request reconcile.Request, instance *nuodbv2alpha1.NuodbInsightsServer, grafanaClusterName string, rm meta.RESTMapper) (reconcile.Result, error) {
	msg := fmt.Sprintf("Reconcile %s in namespace: %s", grafanaClusterName, request.Namespace)
	log.Info(msg)
	ret := reconcile.Result{}
	// Fetch the Nuodb instance
	instance, err := getnuodbv2alpha1NuodbInsightsServerInstance(r, request)
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

	ret, err = reconcileSingleObjectFromYamlFile(r, request, instance, utils.GrafanaCRDYamlFile)
	if err != nil {
		return ret, err
	}

	ret, err = reconcileSingleObjectFromYamlFile(r, request, instance, utils.GrafanaDataSourceCRDYamlFile)
	if err != nil {
		return ret, err
	}

	ret, err = reconcileSingleObjectFromYamlFile(r, request, instance, utils.GrafanaDashboardCRDYamlFile)
	if err != nil {
		return ret, err
	}

	ret, err = reconcileSingleObjectFromYamlFile(r, request, instance, utils.GrafanaServiceAccount)
	if err != nil {
		return ret, err
	}

	ret, err = reconcileSingleObjectFromYamlFile(r, request, instance, utils.GrafanaRole)
	if err != nil {
		return ret, err
	}

	ret, err = reconcileSingleObjectFromYamlFile(r, request, instance, utils.GrafanaRoleBinding)
	if err != nil {
		return ret, err
	}

	ret, err = reconcileSingleObjectFromYamlFile(r, request, instance, utils.GrafanaOperatorDeploymentYamlFile)
	if err != nil {
		return ret, err
	}

	tryCount := 1
	for {
		deployGrafana, err := utils.GetDeployment(r.client, request.Namespace, "grafana-operator")
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}
		}
		if deployGrafana != nil {
			if deployGrafana.Status.ReadyReplicas > 0 {
				break
			}
		}

		if tryCount > 5 {
			return reconcile.Result{Requeue:true}, nil
		}
		tryCount++
		sleepDuration := time.Duration(tryCount) * time.Second
		time.Sleep(sleepDuration)
	}

	ret, err = reconcileGrafanaDataSourceCR(r, request, utils.GrafanaClusterName, rm)
	if err != nil {
		return ret, err
	}

	for dashboardName, dashboardJsonFileName := range utils.GrafanaDashboardsMap {
		ret, err = reconcileGrafanaDashboardCR(r, request, utils.GrafanaClusterName,
			dashboardJsonFileName, dashboardName, rm)
		if err != nil {
			return ret, err
		}
	}

	ret, err = reconcileGrafanaCR(r, request, utils.GrafanaClusterYamlFile, utils.GrafanaClusterName, rm)
	if err != nil {
		return ret, err
	}

	return reconcile.Result{}, err
}

func deleteGrafanaDashboard(thisClient client.Client, namespace string, dashboardName string, rm meta.RESTMapper) error {
	tryCount := 1
	for {
		dashboardCr, err := utils.GetGrafanaDashboardCR(dashboardName, namespace, rm)
		if err != nil {
			if _, ok := err.(*meta.NoKindMatchError); ok {
				return nil
			} else if apierrors.IsNotFound(err) {
				return nil
			} else {
				log.Error(err, "Unable to get GrafanDashboardCR.",
					"GrafanaClusterDataSourceName", utils.GrafanaClusterDataSourceName,
					"namespace", namespace, "name", dashboardName)
				return err
			}
		}
		if dashboardCr != nil {
			err = thisClient.Delete(context.TODO(), dashboardCr)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				} else {
					log.Error(err, "Unable to delete GrafanDashboardCR.",
						"GrafanaClusterDataSourceName", utils.GrafanaClusterDataSourceName,
						"namespace", namespace, "name", dashboardName)
					return err
				}
			}
		}
		if tryCount > 5 {
			msg := "too many retries in GrafanaDeleteDashboard"
			err = apierrors.NewTimeoutError(msg, 5)
			log.Error(err, msg, "namespace", namespace, "name", dashboardName)
		}
		tryCount++
		sleepDuration := time.Duration(tryCount) * time.Second
		time.Sleep(sleepDuration)
	}
}

func deleteGrafanaCluster(thisClient client.Client, namespace string, rm meta.RESTMapper) error {
	msg := fmt.Sprintf("Deleting %s in namespace: %s", utils.GrafanaClusterName, namespace )
	log.Info(msg)

	for dashboardName := range utils.GrafanaDashboardsMap {
		err := deleteGrafanaDashboard(thisClient, namespace, dashboardName, rm)
		if err != nil {
			return err
		}
	}

	datasourceCr, err := utils.GetGrafanaDataSourceCR(utils.GrafanaClusterDataSourceName, namespace, rm)
	if err == nil {
		err = thisClient.Delete(context.TODO(), datasourceCr)
		if err != nil {
			log.Error(err, "Unable to delete GrafanaDataSourceCR.",
				"GrafanaClusterDataSourceName", utils.GrafanaClusterDataSourceName,
				"namespace", namespace)
			return err
		}
	} else {
		if _, ok := err.(*meta.NoKindMatchError); ok {
			log.Info("Unable to find Kind: GrafanaDataSource.  Skipping.")
		} else if !apierrors.IsNotFound(err) {
			log.Error(err, "Unable to get GrafanaDataSourceCR.",
				"GrafanaClusterDataSourceName", utils.GrafanaClusterDataSourceName,
				"namespace", namespace)
			return err
		}
	}

	grafanaCr, err := utils.GetGrafanaCR(utils.GrafanaClusterName, namespace, rm)
	if err != nil {
		if _, ok := err.(*meta.NoKindMatchError); ok {
			log.Info("Unable to find Kind: Grafana.  Skipping.")
		} else if !apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("Unable to find  %s in namespace: %s", utils.GrafanaClusterName, namespace)
			log.Error(err, msg)
			return err
		}
	} else {
		err = thisClient.Delete(context.TODO(), grafanaCr)
		if err != nil {
			msg := fmt.Sprintf("Unable to delete  %s in namespace: %s", utils.GrafanaClusterName, namespace)
			log.Error(err, msg)
			return err
		}
	}

	grafanaOperatorDeployment, err := utils.GetDeployment(thisClient, namespace, "grafana-operator")
	if err == nil {
		err = thisClient.Delete(context.TODO(), grafanaOperatorDeployment)
		if err != nil {
			msg := fmt.Sprintf("Unable to delete %s in namespace: %s", "grafana-operator", namespace )
			log.Error(err, msg)
			return err
		}
	}
	return nil
}

