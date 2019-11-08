package e2e

import (
	goctx "context"
	"crypto/tls"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v7"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"golang.org/x/net/context"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net"
	"net/http"
	operator "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	testutil "nuodb/nuodb-operator/test/e2e/util"
	"strings"
	"testing"
	"time"
)

const (
	ESClusterName = "insights-escluster"  // must match name in insights-server/escluster.yaml
	ESClusterUserSecret = ESClusterName + "-es-elastic-user"
	ESClusterHttpCertsPublic = ESClusterName + "-es-http-certs-public"
	ESClusterServiceHttp = ESClusterName + "-es-http"
)

func verifyAllExpectedPodsExists(t *testing.T, f *framework.Framework, namespace string) {
	podNames := [] string {
		"admin",
		"sm",
		"te",
		"ycsb-load",
		"insights-client",
		"insights-cluster",
		"logstash",
		"grafana",
		"thp",
	}
	//get all running pods in namespace
	pods:=testutil.FindAllPodsInSchema(t,f,namespace)
	var countPods =0
	for _, pod:= range pods{
		for _, item := range podNames {
			if strings.Contains(pod.Name, item){
				countPods++
			}
		}
	}
	t.Log("countpods", countPods)
	assert.Assert(t,countPods>9,)
}

func verifyYcsbContainer(t *testing.T, f *framework.Framework, namespace string) {
	var replicationController  = &corev1.ReplicationController{}
	err := f.Client.Get(context.TODO(), types.NamespacedName{Name: "ycsb-load", Namespace: namespace}, replicationController)
	assert.NilError(t, err)
	repSize := replicationController.Spec.Replicas
	assert.Assert(t,*repSize > 0,)
}

func verifyGrafanaDashboards(t *testing.T, f *framework.Framework,namespace string) {
	listOptions := metav1.ListOptions{
		LabelSelector: "app=grafana",
	}
	podList,err := f.KubeClient.CoreV1().Pods(namespace).List(listOptions)
	if err != nil {
		t.Fatalf(err.Error())
	}

	var grafanaPod *corev1.Pod
	for i := range podList.Items {
		grafanaPod = &podList.Items[i]
	}

	if grafanaPod!=nil {
		command := []string{"cat", "-A", "/var/lib/grafana/grafana.db"}
		testOutput, err := testutil.ExecCommand(f, namespace, grafanaPod.Name, grafanaPod.Spec.Containers[0].Name, command)
		assert.NilError(t, err)
		assert.Assert(t, is.Contains(testOutput, "nuodb-te-resource-states"))
	}

}

func verifyElasticData(t *testing.T, f *framework.Framework,namespace string) {
	esClient, err := GetESClient(t,f,namespace)
	if err != nil {
		t.Fatalf(err.Error())
	}

	templateName := "ic_nuoadminagentlog_template"
	templateSlice := []string{templateName}
	resp, err := esClient.Indices.ExistsTemplate(templateSlice)
	if err != nil {
		t.Fatalf(err.Error())
	}

	if resp.StatusCode == 404 {
		t.Fatalf("Nuoadminagentlog Template not found")
	}
	if resp.IsError(){
		var e map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
			t.Fatalf("Error parsing the response body: %s, %v", err, resp.StatusCode)
		} else {
			// Print the response status and error information.
			t.Fatalf("[%s] %s: %s",
				resp.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
	}

	assert.Equal(t,resp.StatusCode,200)
}

func TestNuodbInsights(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	var (
		namespace               = "nuodb"
		storageMode             = "ephemeral"
		adminCount        int32 = 1
		adminStorageSize        = "5G"
		adminStorageClass       = "local-disk"
		dbName                  = "test1"
		dbUser                  = "dba"
		dbPassword              = "secret"
		smMemory          	    = "500m"
		smCount           int32 = 1
		smCpu             		= "100m"
		smStorageSize           = "20G"
		smStorageClass          = "local-disk"
		engineOptions           = ""
		teCount           int32 = 1
		teMemory          		= "500m"
		teCpu              		= "100m"
		apiServer               = "https://domain:8888"
		container               = "nuodb/nuodb-ce:latest"
	)

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}

	clusterSpec:= operator.NuodbSpec{
		StorageMode:       storageMode,
		InsightsEnabled:   false,
		AdminCount:        adminCount,
		AdminStorageSize:  adminStorageSize,
		AdminStorageClass: adminStorageClass,
		DbName:            dbName,
		DbUser:            dbUser,
		DbPassword:        dbPassword,
		SmMemory:          smMemory,
		SmCount:           smCount,
		SmCpu:             smCpu,
		SmStorageSize:     smStorageSize,
		SmStorageClass:    smStorageClass,
		EngineOptions:     engineOptions,
		TeCount:           teCount,
		TeMemory:          teMemory,
		TeCpu:             teCpu,
		ApiServer:         apiServer,
		Container:         container,
	}

	exampleNuodb := testutil.NewNuodbCluster(namespace, clusterSpec)
	testutil.SetupOperator(t,ctx)
	err = testutil.DeployNuodb(t, ctx, exampleNuodb )
	if err != nil {
		t.Fatal(err)
	}

	f := framework.Global
	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
	if err != nil {
		t.Fatal(err)
	}

	//Create insights cr
	insightsSpec := operator.NuodbInsightsServerSpec{
		ElasticVersion:   "7.3.0",
		ElasticNodeCount: 1,
		KibanaVersion:    "7.3.0",
		KibanaNodeCount:  1,
	}

	//NuoDB Insights-Server
	exampleNuodbInsight := testutil.NewNuodbInsightsCluster(namespace, insightsSpec)

	err = testutil.DeployInsightsServer(t, ctx, exampleNuodbInsight )
	if err != nil {
		t.Fatal(err)
	}

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "insightsserver", Namespace: namespace}, exampleNuodbInsight)
	if err != nil {
		t.Fatal(err)
	}

	insightClientPod := testutil.GetInsightsClientPod(namespace)
	err = testutil.CreateInsightsPods(t,ctx,insightClientPod)
	if err != nil {
		t.Fatal(err)
	}

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "insights-client", Namespace: namespace}, insightClientPod)
	if err != nil {
		t.Fatal(err)
	}

	ycsbwSpec := operator.NuodbYcsbWlSpec{
		DbName: dbName,
		YcsbWorkloadCount: 1,
		YcsbLoadName: "ycsb-load",
		YcsbWorkload: "b",
		YcsbLbPolicy: "",
		YcsbNoOfProcesses: 2,
		YcsbNoOfRows: 10000,
		YcsbNoOfIterations: 0,
		YcsbOpsPerIteration: 10000,
		YcsbMaxDelay: 240000,
		YcsbDbSchema: "User1",
		YcsbContainer: "nuodb/ycsb:latest",
	}

	exampleNuodbYcsbw := testutil.NewNuodbYcsbwCluster(namespace, ycsbwSpec)

	err = testutil.DeployYcsbw(t, ctx, exampleNuodbYcsbw )
	if err != nil {
		t.Fatal(err)
	}

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodbycsbwl", Namespace: namespace}, exampleNuodbYcsbw)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("verifyNuoDbState", func(t *testing.T) { testutil.VerifyAdminState(t, f, namespace, "admin-0", "admin") })
	t.Run("verifyYcsbContainer", func(t *testing.T) { verifyYcsbContainer(t, f, "nuodb")})
	t.Run("verifyDataInElastic", func(t *testing.T) { verifyElasticData(t,f,  "nuodb") })
	t.Run("verifyGrafanaDashboards", func(t *testing.T) { verifyGrafanaDashboards(t, f, "nuodb")})
	t.Run("verifyAllExpectedPodsExists", func(t *testing.T) { verifyAllExpectedPodsExists(t, f, "nuodb")})
}

func GetESClient(t *testing.T, f *framework.Framework, namespace string) (*elasticsearch.Client, error){
	var esClient *elasticsearch.Client
	esClient = nil
	host, err := f.KubeClient.CoreV1().Services(namespace).Get(ESClusterServiceHttp, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	secret, err := f.KubeClient.CoreV1().Secrets(namespace).Get(ESClusterUserSecret, metav1.GetOptions{})
	if err != nil{
		t.Fatal(err)
	}
	var esPassword string
	secretData := secret.Data
	for k,v := range secretData {
		if k == "elastic" {
			esPassword = string(v)
		}
	}

	certSecret, err := f.KubeClient.CoreV1().Secrets(namespace).Get(ESClusterHttpCertsPublic, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	tlsCrt := certSecret.Data["tls.crt"]
	certificate := tls.Certificate{}
	certificate.Certificate = append(certificate.Certificate, tlsCrt)

	tlsConfig := tls.Config{
		MinVersion:tls.VersionTLS11,
		InsecureSkipVerify: true,
		Certificates: []tls.Certificate{certificate},
	}

	url := "https://" + host.Status.LoadBalancer.Ingress[0].Hostname + ":9200"
	config := elasticsearch.Config {
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
	esClient, err = elasticsearch.NewClient(config)
	if err != nil {
		return nil,err
	}
	return esClient,nil
}
