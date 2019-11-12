package utils

import (
	"context"
	"crypto/tls"
	"fmt"
	eck "github.com/elastic/cloud-on-k8s/operators/pkg/apis"
	esv1alpha1 "github.com/elastic/cloud-on-k8s/operators/pkg/apis/elasticsearch/v1alpha1"
	kibanav1alpha1 "github.com/elastic/cloud-on-k8s/operators/pkg/apis/kibana/v1alpha1"
	"github.com/elastic/go-elasticsearch/v7"
	grafanav1alpha1 "github.com/integr8ly/grafana-operator/pkg/apis/integreatly/v1alpha1"
	ocpappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	//routev12 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	policyv1b1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cgretry "k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/policy"
	"net"
	"net/http"
	"nuodb/nuodb-operator/pkg/trace"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strings"
	"time"
)

const (
	NuodbOperatorVersion = "v2.0.2"
	// Default path to the Operator "etc" directory when
	// running the Operator "inside" of a K8s cluster.
	DefaultOperatorEtcDir = "/usr/local/etc/nuodb-operator"

	// relative to the operator repo
	BuildEtcCharts = "build/etc/charts"

	// paths relative to OperatorEtcDir
	ECKAllInOneYamlFileRelPath = "insights-server/eck-0.9.0-all-in-one.yaml"
	ESClusterConfigRelPath = "insights-server/config/elasticsearch"
	ESClusterYamlFileRelPath = "insights-server/escluster.yaml"
	GrafanaConfigRelPath = "insights-server/config/grafana"
	GrafanaOperatorRelPath = "grafana-operator_v2.0.0"
	KibanaYamlFileRelPath = "insights-server/kibana.yaml"
	LogstashChartRelPath = "charts/insights-server/logstash"
	NuodbChartRelPath = "charts/nuodb-helm"
	NuodbYcsbChartRelPath = "charts/nuodb-ycsbwl"

	// paths relative to GrafanaOperatorRelPath
    GrafanaOperatorCRDYamlRelFile = "deploy/crds/Grafana.yaml"
    GrafanaOperatorDashboardCRDYamlRelFile = "deploy/crds/GrafanaDashboard.yaml"
    GrafanaOperatorDataSourceCRDYamlRelFile = "deploy/crds/GrafanaDataSource.yaml"
	GrafanaOperatorDeploymentYamlRelFile = "deploy/operator.yaml"
    GrafanaOperatorRoleRelFile = "deploy/roles/role.yaml"
    GrafanaOperatorRoleBindingRelFile = "deploy/roles/role_binding.yaml"
	GrafanaOperatorServiceAccountRelFile = "deploy/roles/service_account.yaml"

    // paths relative to GrafanaConfigDir
    GrafanaClusterYamlRelFile = "insights-grafana.yaml"
    GrafanaDashboardSystemOverviewJsonRelFile = "dashboards/system-overview.json"
    GrafanaDashboardConnectionsJsonRelFile = "dashboards/connections.json"
    GrafanaDashboardCpuJsonRelFile = "dashboards/cpu.json"
    GrafanaDashboardDiskJsonRelFile = "dashboards/disk.json"
    GrafanaDashboardNuoDBTEResourcesJsonRelFile = "dashboards/nuodb-te-resource-states.json"
    GrafanaDashboardSqlJsonRelFile = "dashboards/sql.json"

	ECKFinalizerName = "ECK.finalizers.nuodbinsightsserver.nuodb.com"
	ElasticNamespace = "elastic-system"
	ElasticSearchGroupVersion = "elasticsearch.k8s.elastic.co/v1alpha1"
	ElasticSearchGroup = "elasticsearch.k8s.elastic.co"
	ElasticSearchVersion = "v1alpha1"
	ElasticSearchKind = "Elasticsearch"
	ESClusterName = "insights-escluster"  // must match name in insights-server/escluster.yaml
	ESClusterUserSecret = ESClusterName + "-es-elastic-user"
	ESClusterHttpCertsInternal = ESClusterName + "-es-http-certs-internal"
	ESClusterHttpCertsPublic = ESClusterName + "-es-http-certs-public"
    ESClusterServiceHttp = ESClusterName + "-es-http"
	ESClusterService = ESClusterName + "-es-http"
	KibanaGroup = "kibana.k8s.elastic.co"
	KibanaVersion = "v1alpha1"
	KibanaKind = "Kibana"
	KibanaClusterName = "kibana"  // must match name in insights-server/kibana.yaml
	KibanaGroupVersion = "kibana.k8s.elastic.co/v1alpha1"
	LogstashClusterName = "insights-logstash"
	GrafanaGroup = "integreatly.org"
	GrafanaVersion = "v1alpha1"
	GrafanaKind = "Grafana"
	GrafanaGroupVersion = "integreatly.org/v1alpha1"
	GrafanaClusterName = "insights-grafana"  // must match name in insights-server/config/grafana/insights-grafana.yaml
	GrafanaClusterDataSourceName = "insights-es-datasource"
	GrafanaDataSourceKind = "GrafanaDataSource"
	GrafanaDashboardKind = "GrafanaDashboard"
	ReconcileRequeueAfterDefault = 10
)

// Global values derived at runtime.
var OperatorEtcDir = GetOperatorEtcDir()
var ECKAllInOneYamlFile = path.Join(OperatorEtcDir, ECKAllInOneYamlFileRelPath)
var ESClusterConfigDir = path.Join(OperatorEtcDir, ESClusterConfigRelPath)
var ESClusterYamlFile = path.Join(OperatorEtcDir, ESClusterYamlFileRelPath)
var GrafanaConfigDir = path.Join(OperatorEtcDir, GrafanaConfigRelPath)
var GrafanaCRDYamlFile = path.Join(GrafanaConfigDir, GrafanaOperatorRelPath, GrafanaOperatorCRDYamlRelFile)
var GrafanaDashboardCRDYamlFile = path.Join(GrafanaConfigDir, GrafanaOperatorRelPath, GrafanaOperatorDashboardCRDYamlRelFile)
var GrafanaDataSourceCRDYamlFile = path.Join(GrafanaConfigDir, GrafanaOperatorRelPath, GrafanaOperatorDataSourceCRDYamlRelFile)
var GrafanaServiceAccount = path.Join(GrafanaConfigDir, GrafanaOperatorRelPath, GrafanaOperatorServiceAccountRelFile)
var GrafanaRole = path.Join(GrafanaConfigDir, GrafanaOperatorRelPath, GrafanaOperatorRoleRelFile)
var GrafanaRoleBinding = path.Join(GrafanaConfigDir, GrafanaOperatorRelPath, GrafanaOperatorRoleBindingRelFile)
var GrafanaOperatorDeploymentYamlFile = path.Join(GrafanaConfigDir, GrafanaOperatorRelPath, GrafanaOperatorDeploymentYamlRelFile)
var GrafanaClusterYamlFile = path.Join(GrafanaConfigDir, GrafanaClusterYamlRelFile)
var GrafanaDashboardSystemOverviewJson = path.Join(GrafanaConfigDir, GrafanaDashboardSystemOverviewJsonRelFile)
var GrafanaDashboardConnectionsJson = path.Join(GrafanaConfigDir, GrafanaDashboardConnectionsJsonRelFile)
var GrafanaDashboardCpuJson = path.Join(GrafanaConfigDir, GrafanaDashboardCpuJsonRelFile)
var GrafanaDashboardDiskJson = path.Join(GrafanaConfigDir, GrafanaDashboardDiskJsonRelFile)
var GrafanaDashboardNuoDBTEResourcesJson = path.Join(GrafanaConfigDir, GrafanaDashboardNuoDBTEResourcesJsonRelFile)
var GrafanaDashboardSqlJson = path.Join(GrafanaConfigDir, GrafanaDashboardSqlJsonRelFile)
var KibanaYamlFile = path.Join(OperatorEtcDir, KibanaYamlFileRelPath)
var LogstashChartDir = path.Join(OperatorEtcDir, LogstashChartRelPath)
var NuodbChartDir = path.Join(OperatorEtcDir, NuodbChartRelPath)
var NuodbYcsbChartDir = path.Join(OperatorEtcDir, NuodbYcsbChartRelPath)

// Depending on where the Operator is executed, the location of the "build/etc" directory
// could be different.  When running the operator "outside" of the K8s Cluster, the
// directory would typically be relative to the root of this Git repo.  When running
// inside of a K8s cluster, the directory is "/usr/local/etc/nuodb-operator".  This
// function will attempt to dynamically locate the "build/etc" directory.  It also
// allows the environment variable "NuodbOperatorEtcDir" to override this function.
func GetOperatorEtcDir() string {
	// NuodbOperatorEtcDir Environment Variable Override
	exPath, envFound := os.LookupEnv("NuodbOperatorEtcDir")
	if envFound {
		return exPath
	}

	// Check the default/normal location.
	if _, err := os.Stat(DefaultOperatorEtcDir); !os.IsNotExist(err) {
		return DefaultOperatorEtcDir
	}

	// Check the current directory
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(path.Join(cwd, BuildEtcCharts)); !os.IsNotExist(err) {
		return cwd + "/build/etc"
	}

	// walk the parents until we find "build/etc/charts"
	parent := path.Dir(cwd)
	for {
		if _, err := os.Stat(path.Join(parent, BuildEtcCharts)); !os.IsNotExist(err) {
			return path.Join(parent, "build/etc")
		}
		parent = path.Dir(parent)
		if parent == "/" {
			return "."
		}
	}
	// Check ancestors of the current directory
	return "."
}

// Map of Grafana dashboard names to JSON strings
var GrafanaDashboardsMap = map[string]string {
	"system-overview": GrafanaDashboardSystemOverviewJson,
	"connections": GrafanaDashboardConnectionsJson,
	"cpu": GrafanaDashboardCpuJson,
	"disk": GrafanaDashboardDiskJson,
	"nuodb-te-resource-states": GrafanaDashboardNuoDBTEResourcesJson,
	"sql": GrafanaDashboardSqlJson,
}

// Logger for "utils"
var log = logf.Log.WithName("utils")

// Helper function to check string from a slice of strings.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// Helper function to remove string from a slice of strings.
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// Get all of the ElasticSearch "*_template" configuration files
func GetESTemplateConfigFiles() (matches []string, err error) {
	filePath := ESClusterConfigDir + "/*_template"
	files, err := filepath.Glob(filePath)
	return files, err
}

// Get all of the ElasticSearch "*_pipeline" configuration files
func GetESPipelineConfigFiles() (matches []string, err error) {
	dirPath := ESClusterConfigDir + "/*_pipeline"
	files, err := filepath.Glob(dirPath)
	return files, err
}

// Get the ElasticSearch public HTTP Certificate data
func GetESClusterHttpCertsPublicData(namespace string) (map[string][]byte, error) {
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return map[string][]byte{}, err
	}
	aClientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return map[string][]byte{}, err
	}
	secret, err := aClientSet.CoreV1().Secrets(namespace).Get(ESClusterHttpCertsPublic, metav1.GetOptions{})
	if err != nil {
		return map[string][]byte{}, err
	}
	return secret.Data, err
}

// Get the ElasticSearch hostname
func GetESHost(namespace string) (string, error) {
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return "", err
	}
	aClientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return "", err
	}
	host, err := aClientSet.CoreV1().Services(namespace).Get(ESClusterServiceHttp, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// Temporary debugging code to work with external "kubefwd" tool,
	// useful when running this Operator "outside" the K8s cluster,
	// to use, uncomment the following line:
	// return ESClusterServiceHttp, nil

	if OperatorEtcDir == DefaultOperatorEtcDir {
		// We are running "inside" the K8s cluster
		return host.Spec.ClusterIP, err
	} else {
		// We are running Outside of the Cluster - use the external load balancer
		if host.Status.LoadBalancer.Ingress == nil {
			err = fmt.Errorf("cannot find ES host Status.LoadBalancer.Ingress")
			return "", err
		}
		return host.Status.LoadBalancer.Ingress[0].Hostname, err
	}
}

// Get the ElasticSearch password
func GetESPassword(namespace string) (string, error) {
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return "", err
	}
	aClientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return "", err
	}
	secret, err := aClientSet.CoreV1().Secrets(namespace).Get(ESClusterUserSecret, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	secretData := secret.Data
	for k,v := range secretData {
		if k == "elastic" {
			return string(v), err
		}
	}
	return "", err
}

// Get the ElasticSearch (GoLang) Client
func GetESClient(namespace string) (*elasticsearch.Client, error) {
	var c *elasticsearch.Client
	c = nil
	esHost, err := GetESHost(namespace)
	if err != nil {
		return c, err
	}

	esPassword, err := GetESPassword(namespace)
	if err != nil {
		return c, err
	}

	certData, err := GetESClusterHttpCertsPublicData(namespace)
	if err != nil {
		return c, err
	}
	tlsCrt := certData["tls.crt"]
	certificate := tls.Certificate{}
	certificate.Certificate = append(certificate.Certificate, tlsCrt)

	tlsConfig := tls.Config{
		MinVersion:tls.VersionTLS11,
		InsecureSkipVerify: true,
		Certificates: []tls.Certificate{certificate},
		//		RootCAs: certPool,
	}

	url := "https://" + esHost + ":9200"
	_config := elasticsearch.Config {
		Addresses: [] string{
			url,
			},
		Username: "elastic",
		Password: esPassword,

		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Second,
			DialContext:           (&net.Dialer{Timeout: time.Second * 3}).DialContext,
			TLSClientConfig: &tlsConfig,
		},
	}

	c, err = elasticsearch.NewClient(_config)
	if err != nil {
		return c, err
	}

	_, err = c.Info()
	if err != nil {
		return c, err
	}

	return c, err
}

// Get the ElasticSearch Cluster IP
func GetESClusterIP(namespace string) (string, error) {
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return "", err
	}
	_clientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return "", err
	}
	serviceName := ESClusterName + "-es-http"
	esService, err := _clientSet.CoreV1().Services(namespace).Get(serviceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	clusterIP := esService.Spec.ClusterIP
	return clusterIP, err
}

// Register the OpenShift types that are supported by this Operator.
func InstallOpenShiftTypes(mgr manager.Manager) error {
	s := mgr.GetScheme()
	var err error=nil
	if err = routev1.AddToScheme(s); err != nil {
		log.Error(err, "Cannot add route/v1 resource")
		return err
	}
	if err = ocpappsv1.AddToScheme(s); err != nil {
		log.Error(err, "Cannot add Openshift apps/v1 resource")
		return err
	}
	if err = apiextensionsv1beta1.AddToScheme(s); err != nil {
		log.Error(err, "Cannot add apiextensionsv1beta1 resource")
		return err
	}
	return err
}

// Register additional K8s types that are supported by this Operator.
func InstallAdditionalTypes(mgr manager.Manager) error {
	s := mgr.GetScheme()
	var err error=nil
	if err = apiextensionsv1beta1.AddToScheme(s); err != nil {
		log.Error(err, "Cannot add apiextensionsv1beta1 resource")
		return err
	}
	if err = policy.AddToScheme(s); err != nil {
		log.Error(err, "Cannot add policy resource")
		return err
	}
	if err = grafanav1alpha1.AddToScheme(s); err != nil {
		log.Error(err, "Cannot add integreatly/v1alpha1 resource")
		return err
	}
	err = InstallOpenShiftTypes(mgr)
	return err
}

// Install the ElasticSearch Cloud on Kubernetes (ECK) Types
func InstallECKTypes(mgr manager.Manager) error {
	s := mgr.GetScheme()
	var err error=nil
	if err = eck.AddToScheme(s); err != nil {
		log.Error(err, "Cannot add ECK resource")
	}
	return err
}

// GetKubeconfigAndNamespace returns the *rest.Config and default namespace
// defined in the kubeconfig at the specified path. If no path is provided,
// returns the default *rest.Config and namespace
func GetKubeconfigAndNamespace(configPath string) (*rest.Config, string, error) {
	var clientConfig clientcmd.ClientConfig
	var apiConfig *clientcmdapi.Config
	var err error

	if configPath != "" {
		apiConfig, err = clientcmd.LoadFromFile(configPath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load user provided kubeconfig: %v", err)
		}
	} else {

		apiConfig, err = clientcmd.NewDefaultClientConfigLoadingRules().Load()
		if err != nil {
			return nil, "", fmt.Errorf("failed to get kubeconfig: %v", err)
		}
	}
	clientConfig = clientcmd.NewDefaultClientConfig(*apiConfig, &clientcmd.ConfigOverrides{})
	kubeconfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", err
	}
	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, "", err
	}
	return kubeconfig, namespace, nil
}

// Get the default Kube config.
func GetDefaultKubeConfig() (*rest.Config, error) {
	kubeconfig, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig, _, err = GetKubeconfigAndNamespace("")
		if err != nil {
			return nil, err
		}
	}
	return kubeconfig, err
}

// Get the K8s Clientset
func GetK8sClientSet() (*kubernetes.Clientset, error) {
	var _clientSet *kubernetes.Clientset
	_clientSet = nil
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Failed to get Config.")
		return _clientSet, ConvertError(err)
	}
	_clientSet, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "Failed to get ClientSet.")
		return _clientSet, ConvertError(err)
	}
	return _clientSet, err
}

// Get the API ClientSet
func GetApiClientSet() (*clientset.Clientset, error) {
	var apiClientSet *clientset.Clientset
	apiClientSet = nil
	kubeconfig, err := GetDefaultKubeConfig()
	apiClientSet, err = clientset.NewForConfig(kubeconfig)
	return apiClientSet, err
}

func GetApiExtensionClientSet() (v1beta1.ApiextensionsV1beta1Interface, error) {
	var apiextensionsClientSet v1beta1.ApiextensionsV1beta1Interface
	apiextensionsClientSet = nil
	apiClientSet, err := GetApiClientSet()
	if err != nil {
		log.Error(err, "Failed to get ClientSet.")
		return apiextensionsClientSet, ConvertError(err)
	}
	apiextensionsClientSet = apiClientSet.ApiextensionsV1beta1()
	return apiextensionsClientSet, err
}

// Get the K8s Rest Mapper
func GetNewRestMapper() (meta.RESTMapper, error) {
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return nil, err
	}
	_clientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	groupResources, err := restmapper.GetAPIGroupResources(_clientSet.Discovery())
	if err != nil {
		return nil, err
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)
	return rm, err
}

// Get the K8s instance for the specified Group/Version/Kind (GVK) in
// the specified name/namespace.
func GetGVKInstance(group string, version string, kind string, namespace string, name string, rm meta.RESTMapper) (*unstructured.Unstructured, error) {
	var _unstructured *unstructured.Unstructured = nil
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		_unstructured = nil
		return _unstructured, err
	}
	dynClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return _unstructured, err
	}
	gvk := schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return _unstructured, err
	}
	_unstructured, err = dynClient.Resource(mapping.Resource).Namespace(namespace).Get(name, metav1.GetOptions{})
	return _unstructured, err
}

func DecodeTemplate(template string, kindName string) (interface{}, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(template), nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Error: decodeTemplate() for Kind: %s", kindName)
		log.Error(err, msg)
		return nil, trace.Wrap(err)
	}
	return obj, err
}


func GetIngress(thisClient client.Client, namespace string, name string) (*extensions.Ingress, error) {
	var ingress = &extensions.Ingress{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, ingress)
	return ingress, err
}

func DecodeIngressTemplate(template string) (*extensions.Ingress, error) {
	obj, err := DecodeTemplate(template, "Ingress")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*extensions.Ingress), err
}

func CreateNamespace(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme, ns *corev1.Namespace) error {
	log.Info("Create", "Namespace", ns.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), ns, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), ns)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetNamespace(name string) (*corev1.Namespace, error) {
	var retNamespace *corev1.Namespace = nil
	aClientSet, err := GetK8sClientSet()
	if err != nil {
		return retNamespace, err
	}
	namespaceInterface := aClientSet.CoreV1().Namespaces()
	retNamespace, err = namespaceInterface.Get(name, metav1.GetOptions{})
	return retNamespace, err
}



func CreateSecret(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme, secret *corev1.Secret) error {
	log.Info("Create", "Secret", secret.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), secret, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), secret)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetSecret(thisClient client.Client, namespace string, name string) (*corev1.Secret, error) {
	var secret = &corev1.Secret{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, secret)
	return secret, err
}

func DecodeSecretTemplate(template string) (*corev1.Secret, error) {
	obj, err := DecodeTemplate(template, "Secret")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*corev1.Secret), err
}

func CreateSecretFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*corev1.Secret, error) {
	var secret *corev1.Secret = nil
	secret, err := DecodeSecretTemplate(template)
	if err != nil {
		return secret, err
	}
	secret.Namespace = namespace
	err = CreateSecret(owner, thisClient, thisScheme, secret)
	if err != nil {
		return secret, trace.Wrap(err)
	}
	return secret, err
}

func CreateRole(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme, role *rbacv1.Role) error {
	log.Info("Create", "Role", role.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), role, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), role)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetRole(thisClient client.Client, namespace string, name string) (*rbacv1.Role, error) {
	var role = &rbacv1.Role{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, role)
	return role, err
}

func CreateRoleBinding(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	roleBinding *rbacv1.RoleBinding) error {
	log.Info("Create", "RoleBinding", roleBinding.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), roleBinding, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), roleBinding)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetRoleBinding(thisClient client.Client, namespace string, name string) (*rbacv1.RoleBinding, error) {
	var roleBinding = &rbacv1.RoleBinding{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace:namespace, Name: name}, roleBinding)
	return roleBinding, err
}

func CreateClusterRole(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	clusterRole *rbacv1.ClusterRole) error {
	log.Info("Create", "ClusterRole", clusterRole.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), clusterRole, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), clusterRole)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetClusterRole(thisClient client.Client, namespace string, name string) (*rbacv1.ClusterRole, error) {
	var clusterRole = &rbacv1.ClusterRole{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, clusterRole)
	return clusterRole, err
}

func CreateClusterRoleBinding(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	clusterRoleBinding *rbacv1.ClusterRoleBinding) error {
	log.Info("Create", "ClusterRoleBinding", clusterRoleBinding.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), clusterRoleBinding, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), clusterRoleBinding)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetClusterRoleBinding(thisClient client.Client, namespace string, name string) (*rbacv1.ClusterRoleBinding, error) {
	var clusterRoleBinding = &rbacv1.ClusterRoleBinding{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, clusterRoleBinding)
	return clusterRoleBinding, err
}

func CreateServiceAccount(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	serviceAccount *corev1.ServiceAccount) error {
	log.Info("Create", "ServiceAccount", serviceAccount.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), serviceAccount, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), serviceAccount)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetServiceAccount(thisClient client.Client, namespace string, name string) (*corev1.ServiceAccount, error) {
	var serviceAccount = &corev1.ServiceAccount{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, serviceAccount)
	return serviceAccount, err
}

func DecodeServiceAccountTemplate(template string) (*corev1.ServiceAccount, error) {
	obj, err := DecodeTemplate(template, "ServiceAccount")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*corev1.ServiceAccount), err
}

func CreateServiceAccountFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*corev1.ServiceAccount, error) {
	var serviceAccount *corev1.ServiceAccount = nil
	serviceAccount, err := DecodeServiceAccountTemplate(template)
	if err != nil {
		return serviceAccount, err
	}
	serviceAccount.Namespace = namespace
	err = CreateServiceAccount(owner, thisClient, thisScheme, serviceAccount)
	if err != nil {
		return serviceAccount, trace.Wrap(err)
	}
	return serviceAccount, err
}

func CreateStatefulSet(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	statefulSet *appsv1.StatefulSet) error {
	log.Info("Create", "StatefulSet", statefulSet.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), statefulSet, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), statefulSet)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetStatefulSet(thisClient client.Client, namespace string, name string) (*appsv1.StatefulSet, error) {
	var statefulSet = &appsv1.StatefulSet{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, statefulSet)
	return statefulSet, err
}

func DecodeStatefulSetTemplate(template string) (*appsv1.StatefulSet, error) {
	obj, err := DecodeTemplate(template, "StatefulSet")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*appsv1.StatefulSet), err
}

func CreateStatefulSetFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*appsv1.StatefulSet, error) {
	var statefulSet *appsv1.StatefulSet = nil
	statefulSet, err := DecodeStatefulSetTemplate(template)
	if err != nil {
		return statefulSet, err
	}
	statefulSet.Namespace = namespace
	err = CreateStatefulSet(owner, thisClient, thisScheme, statefulSet)
	if err != nil {
		return statefulSet, trace.Wrap(err)
	}
	return statefulSet, err
}

func CreateStatefulSetV1beta2(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	statefulSet *v1beta2.StatefulSet) error {
	log.Info("Create", "StatefulSet", statefulSet.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), statefulSet, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), statefulSet)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetStatefulSetV1beta2(thisClient client.Client, namespace string, name string) (*v1beta2.StatefulSet, error) {
	var statefulSet = &v1beta2.StatefulSet{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, statefulSet)
	return statefulSet, err
}

func DecodeStatefulSetV1beta2Template(template string) (*v1beta2.StatefulSet, error) {
	obj, err := DecodeTemplate(template, "StatefulSet")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*v1beta2.StatefulSet), err

}

func CreateStatefulSetV1beta2FromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*v1beta2.StatefulSet, error) {
	var statefulSet *v1beta2.StatefulSet = nil
	statefulSet, err := DecodeStatefulSetV1beta2Template(template)
	if err != nil {
		return statefulSet, err
	}
	statefulSet.Namespace = namespace
	err = CreateStatefulSetV1beta2(owner, thisClient, thisScheme, statefulSet)
	if err != nil {
		return statefulSet, trace.Wrap(err)
	}
	return statefulSet, err
}


func CreateDeployment(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	deployment *appsv1.Deployment) error {
	log.Info("Create", "Deployment", deployment.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	err := controllerutil.SetControllerReference(owner.(metav1.Object), deployment, thisScheme)
	if err != nil {
		return trace.Wrap(err)
	}
	err = thisClient.Create(context.TODO(), deployment)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}
func GetDeployment(thisClient client.Client, namespace string, name string) (*appsv1.Deployment, error) {
	var deployment = &appsv1.Deployment{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, deployment)
	return deployment, err
}

func DecodeDeploymentTemplate(template string) (*appsv1.Deployment, error) {
	obj, err := DecodeTemplate(template, "Deployment")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*appsv1.Deployment), err
}

func CreateDeploymentFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*appsv1.Deployment, error) {
	var deployment *appsv1.Deployment = nil
	deployment, err := DecodeDeploymentTemplate(template)
	if err != nil {
		return deployment, err
	}
	deployment.Namespace = namespace
	err = CreateDeployment(owner, thisClient, thisScheme, deployment)
	if err != nil {
		return deployment, trace.Wrap(err)
	}
	return deployment, err
}

func CreateService(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	service *corev1.Service) error {
	log.Info("Create", "Service", service.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), service, thisScheme); err != nil {
		return trace.Wrap(err)
	}
	err := thisClient.Create(context.TODO(), service)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetService(thisClient client.Client, namespace string, name string) (*corev1.Service, error) {
	var service = &corev1.Service{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, service)
	return service, err
}

func DecodeServiceTemplate(template string) (*corev1.Service, error) {
	obj, err := DecodeTemplate(template, "Service")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*corev1.Service), err
}

func CreateServiceFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*corev1.Service, error) {
	var service *corev1.Service = nil
	service, err := DecodeServiceTemplate(template)
	if err != nil {
		return service, err
	}
	service.Namespace = namespace
	err = CreateService(owner, thisClient, thisScheme, service)
	if err != nil {
		return service, trace.Wrap(err)
	}
	return service, err
}

func CreatePod(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme, pod *corev1.Pod) error {
	log.Info("Create", "Pod", pod.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), pod, thisScheme); err != nil {
		return trace.Wrap(err)
	}
	err := thisClient.Create(context.TODO(), pod)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetPod(thisClient client.Client, namespace string, name string) (*corev1.Pod, error) {
	var pod = &corev1.Pod{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, pod)
	return pod, err
}

func DecodePodTemplate(template string) (*corev1.Pod, error) {
	obj, err := DecodeTemplate(template, "Pod")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*corev1.Pod), err
}

func CreatePodFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*corev1.Pod, error) {
	var pod *corev1.Pod = nil
	pod, err := DecodePodTemplate(template)
	if err != nil {
		return pod, err
	}
	pod.Namespace = namespace
	err = CreatePod(owner, thisClient, thisScheme, pod)
	if err != nil {
		return pod, trace.Wrap(err)
	}
	return pod, err
}

func CreateConfigMap(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	configMap *corev1.ConfigMap) error {
	log.Info("Create", "ConfigMap", configMap.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), configMap, thisScheme); err != nil {
		return trace.Wrap(err)
	}
	err := thisClient.Create(context.TODO(), configMap)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetConfigMap(thisClient client.Client, namespace string, name string) (*corev1.ConfigMap, error) {
	var configMap = &corev1.ConfigMap{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, configMap)
	return configMap, err
}

func DecodeConfigMapTemplate(template string) (*corev1.ConfigMap, error) {
	obj, err := DecodeTemplate(template, "ConfigMap")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*corev1.ConfigMap), err
}

func CreateConfigMapFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*corev1.ConfigMap, error) {
	var configMap *corev1.ConfigMap = nil
	configMap, err := DecodeConfigMapTemplate(template)
	if err != nil {
		return configMap, err
	}
	configMap.Namespace = namespace
	err = CreateConfigMap(owner, thisClient, thisScheme, configMap)
	if err != nil {
		return configMap, trace.Wrap(err)
	}
	return configMap, err
}

func CreateRoute(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme, route *routev1.Route) error {
	log.Info("Create", "Route", route.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), route, thisScheme); err != nil {
		return trace.Wrap(err)
	}
	err := thisClient.Create(context.TODO(), route)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetRoute(thisClient client.Client, namespace string, name string) (*routev1.Route, error) {
	var route = &routev1.Route{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, route)
	return route, err
}

func DecodeRouteTemplate(template string) (*routev1.Route, error) {
	obj, err := DecodeTemplate(template, "Route")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*routev1.Route), err
}

func CreateRouteFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*routev1.Route, error) {
	var route *routev1.Route = nil
	route, err := DecodeRouteTemplate(template)
	if err != nil {
		return route, err
	}
	route.Namespace = namespace
	err = CreateRoute(owner, thisClient, thisScheme, route)
	if err != nil {
		return route, trace.Wrap(err)
	}
	return route, err
}

func CreateReplicationController(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	replicationController *corev1.ReplicationController) error {
	log.Info("Create", "ReplicationController", replicationController.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), replicationController, thisScheme); err != nil {
		return trace.Wrap(err)
	}
	err := thisClient.Create(context.TODO(), replicationController)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetReplicationController(thisClient client.Client, namespace string,
	name string) (*corev1.ReplicationController, error) {
	var replicationController = &corev1.ReplicationController{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, replicationController)
	return replicationController, err
}

func DecodeReplicationControllerTemplate(template string) (*corev1.ReplicationController, error) {
	obj, err := DecodeTemplate(template, "ReplicationController")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*corev1.ReplicationController), err
}

func CreateReplicationControllerFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*corev1.ReplicationController, error) {
	var replicationController *corev1.ReplicationController = nil
	replicationController, err := DecodeReplicationControllerTemplate(template)
	if err != nil {
		return replicationController, err
	}
	replicationController.Namespace = namespace
	err = CreateReplicationController(owner, thisClient, thisScheme, replicationController)
	if err != nil {
		return replicationController, trace.Wrap(err)
	}
	return replicationController, err
}

func CreatePodDisruptionBudgetV1b1(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	podDisruptionBudget *policyv1b1.PodDisruptionBudget) error {
	log.Info("Create", "PodDisruptionBudget", podDisruptionBudget.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), podDisruptionBudget, thisScheme); err != nil {
		return trace.Wrap(err)
	}
	err := thisClient.Create(context.TODO(), podDisruptionBudget)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func DecodePodDisruptionBudgetV1b1Template(template string) (*policyv1b1.PodDisruptionBudget, error) {
	obj, err := DecodeTemplate(template, "PodDisruptionBudget")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*policyv1b1.PodDisruptionBudget), err
}

func CreatePodDisruptionBudgetV1b1FromTemplate(owner runtime.Object, thisClient client.Client,
	thisScheme *runtime.Scheme, template string, namespace string) (*policyv1b1.PodDisruptionBudget, error) {
	var podDisruptionBudget *policyv1b1.PodDisruptionBudget = nil
	podDisruptionBudget, err := DecodePodDisruptionBudgetV1b1Template(template)
	if err != nil {
		return podDisruptionBudget, err
	}
	podDisruptionBudget.Namespace = namespace
	err = CreatePodDisruptionBudgetV1b1(owner, thisClient, thisScheme, podDisruptionBudget)
	if err != nil {
		return podDisruptionBudget, trace.Wrap(err)
	}
	return podDisruptionBudget, err
}

func CreateDaemonSet(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	daemonSet *appsv1.DaemonSet) error {
	log.Info("Create", "DaemonSet", daemonSet.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), daemonSet, thisScheme); err != nil {
		return trace.Wrap(err)
	}
	err := thisClient.Create(context.TODO(), daemonSet)
	if err != nil {
		return trace.Wrap(err)
	}
	return err
}

func GetDaemonSet(thisClient client.Client, namespace string, name string) (*appsv1.DaemonSet, error) {
	var daemonSet = &appsv1.DaemonSet{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, daemonSet)
	return daemonSet, err
}

func DecodeDaemonSetTemplate(template string) (*appsv1.DaemonSet, error) {
	obj, err := DecodeTemplate(template, "DaemonSet")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*appsv1.DaemonSet), err
}

func CreateDaemonSetFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*appsv1.DaemonSet, error) {
	var daemonSet *appsv1.DaemonSet = nil
	daemonSet, err := DecodeDaemonSetTemplate(template)
	if err != nil {
		return daemonSet, err
	}
	daemonSet.Namespace = namespace
	err = CreateDaemonSet(owner, thisClient, thisScheme, daemonSet)
	if err != nil {
		return daemonSet, trace.Wrap(err)
	}
	return daemonSet, err
}

// DeploymentConfig is an OpenShift type.
func CreateDeploymentConfig(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	deploymentConfig *ocpappsv1.DeploymentConfig) error {
	log.Info("Create", "DeploymentConfig", deploymentConfig.Name)
	_, ok := owner.(runtime.Object)
	if !ok {
		return fmt.Errorf("is not a %T a runtime.Object, cannot set controller reference", owner)
	}
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), deploymentConfig, thisScheme); err != nil {
		err = trace.Wrap(err)
		return err
	}
	err := thisClient.Create(context.TODO(), deploymentConfig)
	if err != nil {
		err = trace.Wrap(err)
		return err
	}
	return err
}

func GetDeploymentConfig(thisClient client.Client, namespace string, name string) (*ocpappsv1.DeploymentConfig, error) {
	var deploymentConfig = &ocpappsv1.DeploymentConfig{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, deploymentConfig)
	return deploymentConfig, err
}

func DecodeDeploymentConfigTemplate(template string) (*ocpappsv1.DeploymentConfig, error) {
	obj, err := DecodeTemplate(template, "DeploymentConfig")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return obj.(*ocpappsv1.DeploymentConfig), err
}

func CreateDeploymentConfigFromTemplate(owner runtime.Object, thisClient client.Client, thisScheme *runtime.Scheme,
	template string, namespace string) (*ocpappsv1.DeploymentConfig, error) {
	var deploymentConfig *ocpappsv1.DeploymentConfig = nil
	deploymentConfig, err := DecodeDeploymentConfigTemplate(template)
	if err != nil {
		return deploymentConfig, err
	}
	deploymentConfig.Namespace = namespace
	if err := controllerutil.SetControllerReference(owner.(metav1.Object), deploymentConfig, thisScheme); err != nil {
		return deploymentConfig, trace.Wrap(err)
	}
	err = CreateDeploymentConfig(owner, thisClient, thisScheme, deploymentConfig)
	if err != nil {
		return deploymentConfig, trace.Wrap(err)
	}
	return deploymentConfig, err
}

func GetElasticsearchCR(name string, ns string, rm meta.RESTMapper) (*esv1alpha1.Elasticsearch, error) {
	var cr = &esv1alpha1.Elasticsearch{}
	uu2, err := GetGVKInstance(ElasticSearchGroup, ElasticSearchVersion, ElasticSearchKind, ns, name, rm)
	if err != nil {
		cr = nil
		return cr, err
	}
	// convert unstructured.Unstructured to Elasticsearch CR
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		cr = nil
		return cr, err
	}
	return cr, err
}

func GetKibanaCR(name string, ns string, rm meta.RESTMapper) (*kibanav1alpha1.Kibana, error) {
	var cr = &kibanav1alpha1.Kibana{}
	uu2, err := GetGVKInstance(KibanaGroup, KibanaVersion, KibanaKind, ns, name, rm)
	if err != nil {
		cr = nil
		return cr, err
	}
	// convert unstructured.Unstructured to Elasticsearch CR
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		cr = nil
		return cr, err
	}
	return cr, err
}

func GetGrafanaCR(name string, ns string, rm meta.RESTMapper) (*grafanav1alpha1.Grafana, error) {
	var cr = &grafanav1alpha1.Grafana{}
	uu2, err := GetGVKInstance(GrafanaGroup, GrafanaVersion, GrafanaKind, ns, name, rm)
	if err != nil {
		cr = nil
		return cr, err
	}
	// convert unstructured.Unstructured to Elasticsearch CR
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		cr = nil
		return cr, err
	}
	return cr, err
}

func GetGrafanaDataSourceCR(name string, ns string, rm meta.RESTMapper) (*grafanav1alpha1.GrafanaDataSource, error) {
	var cr = &grafanav1alpha1.GrafanaDataSource{}
	uu2, err := GetGVKInstance(GrafanaGroup, GrafanaVersion, GrafanaDataSourceKind, ns, name, rm)
	if err != nil {
		cr = nil
		return cr, err
	}
	// convert unstructured.Unstructured to Elasticsearch CR
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		cr = nil
		return cr, err
	}
	return cr, err
}

func GetGrafanaDashboardCR(name string, ns string, rm meta.RESTMapper) (*grafanav1alpha1.GrafanaDashboard, error) {
	var cr = &grafanav1alpha1.GrafanaDashboard{}
	uu2, err := GetGVKInstance(GrafanaGroup, GrafanaVersion, GrafanaDashboardKind, ns, name, rm)
	if err != nil {
		cr = nil
		return cr, err
	}
	// convert unstructured.Unstructured to Elasticsearch CR
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		cr = nil
		return cr, err
	}
	return cr, err
}

func CreateElasticsearchCR(es runtime.Object, ns string, ownRefs []metav1.OwnerReference,
	rm meta.RESTMapper) (*esv1alpha1.Elasticsearch, error) {
	var cr = &esv1alpha1.Elasticsearch{}
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return cr, err
	}
	dynClient, err := dynamic.NewForConfig(kubeconfig)

	// convert the runtime.Object to unstructured.Unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(es)
	if err != nil {
		return cr, err
	}

	gvk := es.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}

	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return cr, err
	}
	var uu = unstructured.Unstructured{}
	uu.Object = unstructuredObj
	uu.SetOwnerReferences(ownRefs)

	// Create Object
	uu2, err := dynClient.Resource(mapping.Resource).Namespace(ns).Create(&uu, metav1.CreateOptions{})
	if err != nil {
		return cr, err
	}

	// Convert unstructured.Unstructured to Elasticsearch
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		return cr, err
	}

	return cr, err
}

func CreateKibanaCR(kibana runtime.Object, ns string, ownRefs []metav1.OwnerReference,
	rm meta.RESTMapper) (*kibanav1alpha1.Kibana, error) {
	var cr = &kibanav1alpha1.Kibana{}
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return cr, err
	}
	dynClient, err := dynamic.NewForConfig(kubeconfig)

	// convert the runtime.Object to unstructured.Unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(kibana)
	if err != nil {
		return cr, err
	}

	gvk := kibana.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}

	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return cr, err
	}
	var uu = unstructured.Unstructured{}
	uu.Object = unstructuredObj
	uu.SetOwnerReferences(ownRefs)

	// Create Object
	uu2, err := dynClient.Resource(mapping.Resource).Namespace(ns).Create(&uu, metav1.CreateOptions{})
	if err != nil {
		return cr, err
	}

	// Convert unstructured.Unstructured to Elasticsearch
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		return cr, err
	}

	return cr, err
}

func CreateGrafanaCR(grafana runtime.Object, ns string, ownRefs []metav1.OwnerReference,
	rm meta.RESTMapper) (*grafanav1alpha1.Grafana, error) {
	var cr = &grafanav1alpha1.Grafana{}
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return cr, err
	}
	dynClient, err := dynamic.NewForConfig(kubeconfig)

	// convert the runtime.Object to unstructured.Unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(grafana)
	if err != nil {
		return cr, err
	}

	gvk := grafana.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return cr, err
	}
	var uu = unstructured.Unstructured{}
	uu.Object = unstructuredObj
	uu.SetOwnerReferences(ownRefs)

	// Create Object
	uu2, err := dynClient.Resource(mapping.Resource).Namespace(ns).Create(&uu, metav1.CreateOptions{})
	if err != nil {
		return cr, err
	}

	// Convert unstructured.Unstructured to Elasticsearch
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		return cr, err
	}

	return cr, err
}

func CreateGrafanaDataSourceCR(cr *grafanav1alpha1.GrafanaDataSource, ns string, ownRefs []metav1.OwnerReference,
	rm meta.RESTMapper) (*grafanav1alpha1.GrafanaDataSource, error) {
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return cr, err
	}
	dynClient, err := dynamic.NewForConfig(kubeconfig)

	// convert the runtime.Object to unstructured.Unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		return cr, err
	}

	gvk := cr.GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return cr, err
	}
	var uu = unstructured.Unstructured{}
	uu.Object = unstructuredObj
	uu.SetOwnerReferences(ownRefs)

	// Create Object
	uu2, err := dynClient.Resource(mapping.Resource).Namespace(ns).Create(&uu, metav1.CreateOptions{})
	if err != nil {
		return cr, err
	}

	// Convert unstructured.Unstructured to Elasticsearch
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		return cr, err
	}

	return cr, err
}

func CreateGrafanaDashboardCR(cr *grafanav1alpha1.GrafanaDashboard, ns string, ownRefs []metav1.OwnerReference,
	rm meta.RESTMapper) (*grafanav1alpha1.GrafanaDashboard, error) {
	kubeconfig, err := GetDefaultKubeConfig()
	if err != nil {
		return cr, err
	}
	dynClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return cr, err
	}

	// convert the runtime.Object to unstructured.Unstructured
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cr)
	if err != nil {
		return cr, err
	}

	gvk := cr.GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	// TODO: Check err
	mapping, err := rm.RESTMapping(gk, gvk.Version)
	if err != nil {
		return cr, err
	}
	var uu = unstructured.Unstructured{}
	uu.Object = unstructuredObj
	uu.SetOwnerReferences(ownRefs)

	// Create Object
	uu2, err := dynClient.Resource(mapping.Resource).Namespace(ns).Create(&uu, metav1.CreateOptions{})
	if err != nil {
		return cr, err
	}

	// Convert unstructured.Unstructured to Elasticsearch
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uu2.Object, cr)
	if err != nil {
		return cr, err
	}

	return cr, err
}

func GetElasticsearch(thisClient client.Client, namespace string, name string) (*esv1alpha1.Elasticsearch, error) {
	var es = &esv1alpha1.Elasticsearch{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, es)
	return es, err
}

func GetKibana(thisClient client.Client, namespace string, name string) (*kibanav1alpha1.Kibana, error) {
	var kibana = &kibanav1alpha1.Kibana{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, kibana)
	return kibana, err
}

func GetGrafana(thisClient client.Client, namespace string, name string) (*grafanav1alpha1.Grafana, error) {
	var grafana = &grafanav1alpha1.Grafana{}
	err := thisClient.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: name}, grafana)
	return grafana, err
}

func GetCRD(crdName string) (*apiextensionsv1beta1.CustomResourceDefinition, error) {
	var crd *apiextensionsv1beta1.CustomResourceDefinition
	crd = nil
	apiextentionsClientSet, err := GetApiExtensionClientSet()
	if err != nil {
		return crd, err
	}
	crd, err = apiextentionsClientSet.CustomResourceDefinitions().Get(crdName, metav1.GetOptions{})
	return crd, err
}

func GetApiResources(groupVersion string) (*metav1.APIResourceList, error) {
	var apiResources *metav1.APIResourceList
	aClientSet, err := GetApiClientSet()
	if err != nil {
		return apiResources, err
	}
	apiResources, err = aClientSet.ServerResourcesForGroupVersion(groupVersion)
	return apiResources, err
}

// Parse byte array from a YAML file.  The YAML file can contain multiple K8s
// types, seperated by "---".  Then decode each type into a runtime.Object,
// which is returned in a array.
func ParseK8sYaml(fileR []byte) []runtime.Object {
	acceptedK8sTypes := regexp.MustCompile(
		`(Role|ClusterRole|RoleBinding|ClusterRoleBinding|ServiceAccount|CustomResourceDefinition|Namespace|StatefulSet|Secret|Deployment|Elasticsearch|Kibana|Grafana)`)
	fileAsString := string(fileR[:])
	sepYamlfiles := strings.Split(fileAsString, "---")
	retVal := make([]runtime.Object, 0, len(sepYamlfiles))

	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, groupVersionKind, err := decode([]byte(f), nil, nil)
		if err != nil {
			log.Error(err, "Internal Error while decoding YAML object.")
			continue
		}
		if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
			log.Error(err, "Internal Error YAML contained K8s object types which are not supported! Skipping object with type: %s", groupVersionKind.Kind)
		} else {
			retVal = append(retVal, obj)
		}
	}
	return retVal
}

func GetMultipleRuntimeObjectFromYamlFile(yamlFilename string) ([]runtime.Object, error) {
	yamlFile, err := ioutil.ReadFile(yamlFilename)
	if err != nil {
		return nil, err
	}
	runtimeObjects := ParseK8sYaml(yamlFile)
	if len(runtimeObjects) == 0 {
		msg := fmt.Sprintf("Yaml '%s' has no parsable runtime object.", yamlFile)
		log.Error(err, msg)
		return nil, trace.Wrap(err)
	}
	if len(runtimeObjects) < 1 {
		msg := fmt.Sprintf("Yaml '%s' has less than one runtime object.", yamlFile)
		log.Error(err, msg)
		return nil, trace.Wrap(err)
	}
	return runtimeObjects, err
}

func GetSingleRuntimeObjectFromYamlFile(yamlFilename string) (runtime.Object, error) {
	yamlFile, err := ioutil.ReadFile(yamlFilename)
	if err != nil {
		return nil, err
	}
	runtimeObjects := ParseK8sYaml(yamlFile)
	if len(runtimeObjects) == 0 {
		msg := fmt.Sprintf("Yaml '%s' has no parsable runtime object.", yamlFile)
		log.Error(err, msg)
		return nil, trace.Wrap(err)
	}
	if len(runtimeObjects) > 1 {
		msg := fmt.Sprintf("Yaml '%s' has more than one runtime object.", yamlFile)
		log.Error(err, msg)
		return nil, trace.Wrap(err)
	}
	return runtimeObjects[0], err
}

func AddLogstashServiceAccountsToSCC(namespaces *corev1.NamespaceList, sccName string) error {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Failed to get Config.")
		return err
	}
	c, err := securityv1.NewForConfig(cfg)
	err = cgretry.RetryOnConflict(cgretry.DefaultRetry, func() error {
		scc, err := c.SecurityContextConstraints().Get(sccName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		groups := []string{}
		for _, name := range scc.Groups {
			if !strings.Contains(name, "insights-server-release-logstash") {
				groups = append(groups, name)
			}
		}
		for _, ns := range namespaces.Items {
			if strings.HasPrefix(ns.Name, "insights-server-release-logstash") {
				groups = append(groups, fmt.Sprintf("system:serviceaccounts:%s", ns.Name))
			}
		}
		scc.Groups = groups
		if _, err := c.SecurityContextConstraints().Update(scc); err != nil {
			return err
		}
		return nil
	})
	return err
}
