package util

import (
	goctx "context"
	"flag"
	"github.com/gravitational/trace"
	"gotest.tools/assert"
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	corev1 "k8s.io/api/core/v1"
	"nuodb/nuodb-operator/pkg/apis"
	nuodb "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"

	"bytes"
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// Time constants.
const (
	RetryInterval                 = time.Second * 5
	Timeout                       = time.Second * 90
	StatefulSetTimeout            = time.Second * 180
	InsightsServerTimeout         = time.Second * 540
	CleanupRetryInterval          = time.Second * 1
	CleanupTimeout                = time.Second * 15
	InsightsServerCleanupTimeout  = time.Second * 120
)

func NewNuodbAdmin(namespace string, adminSpec nuodb.NuodbAdminSpec) *nuodb.NuodbAdmin {
	name := "nuoadmin"
	return &nuodb.NuodbAdmin{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: adminSpec,
	}
}

func NewNuodbDatabase(namespace string, nuodbSpec nuodb.NuodbSpec) *nuodb.Nuodb {
	name := "nuodb"
	return &nuodb.Nuodb{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: nuodbSpec,
	}
}

// return 'true' if the string is the name of a 'true' boolean flag.
// Otherwise return 'false'
func IsBoolFlagTrue(name string) bool {
	type BoolFlag interface {
		IsBoolFlag() bool
	}
	localOperatorFlag := flag.Lookup(name)
	if localOperatorFlag == nil {
		return false
	}
	getter := localOperatorFlag.Value.(flag.Getter)
	if bf, ok := localOperatorFlag.Value.(BoolFlag); ok && bf.IsBoolFlag() {
		val, ok := getter.Get().(bool)
		if ok && val {
			return true
		}
	}
	return false
}

// SetupOperator installs the operator and ensures that the deployment is successful.
func SetupOperator(t *testing.T, ctx *framework.TestCtx) {
	clusterList := &nuodb.Nuodb{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Nuodb",
			APIVersion: "nuodb.com/v2alpha1",
		},
	}

	err := framework.AddToFrameworkScheme(apis.AddToScheme, clusterList)
	assert.NilError(t, err, "failed to add custom resource scheme to framework")

	if IsBoolFlagTrue("localOperator") {
		t.Log("Testing with local Operator.")
		return
	}

	err = ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: CleanupTimeout, RetryInterval: CleanupRetryInterval})
	assert.NilError(t, err, "failed to initialize cluster resources")

	t.Log("Initialized cluster resources")

	namespace, err := ctx.GetNamespace()
	assert.NilError(t, err)

	f := framework.Global

	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "nuodb-operator", 1, RetryInterval, Timeout)
	assert.NilError(t, err)
	time.Sleep(time.Second * 5) // Temporary workaround until the root cause of DB-30412 is found.
}

// DeployNuodbAdmin creates a custom resource and checks if the
// admin statefulset is deployed successfully.
func DeployNuodbAdmin(t *testing.T, ctx *framework.TestCtx, nuodbAdmin *nuodb.NuodbAdmin) {
	f := framework.Global

	err := f.Client.Create(goctx.TODO(), nuodbAdmin, &framework.CleanupOptions{TestContext: ctx, Timeout: CleanupTimeout, RetryInterval: CleanupRetryInterval})
	if err != nil {
		t.Log(trace.Wrap(err))
	}
	assert.NilError(t, err)

	err = WaitForStatefulSet(t, f.KubeClient, nuodbAdmin.Namespace, "admin", 1, RetryInterval, StatefulSetTimeout)
	assert.NilError(t, err)
}

// DeployNuodb creates a custom resource and checks if the
// TE/SM resources are deployed correctly
func DeployNuodb(t *testing.T, ctx *framework.TestCtx, nuodb *nuodb.Nuodb) {
	f := framework.Global

	err := f.Client.Create(goctx.TODO(), nuodb, &framework.CleanupOptions{TestContext: ctx, Timeout: CleanupTimeout, RetryInterval: CleanupRetryInterval})
	assert.NilError(t, err)

	err = WaitForStatefulSet(t, f.KubeClient, nuodb.Namespace, "nuodb-sm", 1, RetryInterval, StatefulSetTimeout)
	assert.NilError(t, err)

	err = WaitForDeployment(t, f.KubeClient, nuodb.Namespace, "nuodb-te", 1, RetryInterval, StatefulSetTimeout)
	assert.NilError(t, err)
}

func NewNuodbInsightsCluster(namespace string, insightsSpec nuodb.NuodbInsightsServerSpec) *nuodb.NuodbInsightsServer {
	name := "insightsserver"
	return &nuodb.NuodbInsightsServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: insightsSpec,
	}
}

func DeployInsightsServer(t *testing.T, ctx *framework.TestCtx, nuodbInsights *nuodb.NuodbInsightsServer) error {
	f := framework.Global

	err := f.Client.Create(goctx.TODO(), nuodbInsights, &framework.CleanupOptions{TestContext: ctx, Timeout: InsightsServerCleanupTimeout, RetryInterval: CleanupRetryInterval})
	if err != nil {
		return err
	}

	err = WaitForStatefulSet(t, f.KubeClient, nuodbInsights.Namespace, "insights-server-release-logstash", 1, RetryInterval, InsightsServerTimeout)
	if err != nil {
		t.Fatal(err)
	}

	return nil
}

func NewNuodbYcsbwCluster(namespace string, ycsbwSpec nuodb.NuodbYcsbWlSpec) *nuodb.NuodbYcsbWl {
	name := "nuodbycsbwl"
	return &nuodb.NuodbYcsbWl{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ycsbwSpec,
	}
}

func DeployYcsbw(t *testing.T, ctx *framework.TestCtx, nuodbYcsbw *nuodb.NuodbYcsbWl) error {
	f := framework.Global

	err := f.Client.Create(goctx.TODO(), nuodbYcsbw, &framework.CleanupOptions{TestContext: ctx, Timeout: CleanupTimeout, RetryInterval: CleanupRetryInterval})
	assert.NilError(t, err)

	WaitForRC(t, f, nuodbYcsbw.Namespace)
	t.Log("YCSBW Created")
	return nil

}

func WaitForRC(t *testing.T, f *framework.Framework, namespace string) {
	rcClient := f.KubeClient.CoreV1().ReplicationControllers(namespace)
	var err1 error
	if err := wait.PollImmediate(RetryInterval, Timeout, func() (bool, error) {
		newRC, err1 := rcClient.Get("ycsb-load", metav1.GetOptions{})
		if err1 != nil {
			return false, nil
		}
		return newRC.Status.Replicas == 1, nil
	}); err != nil {
		t.Fatalf("Failed to verify .Status.Replicas is equal to .Spec.Replicas for rc %s: %v", "ycsb-load", err1)
	}
}

// WaitForStatefulSet checks and waits for a given statefulset to be in ready.
func WaitForDeployment(t *testing.T, kubeclient kubernetes.Interface, namespace, name string, replicas int, retryInterval, timeout time.Duration) error {
	return e2eutil.WaitForDeployment(t, kubeclient, namespace, name, replicas, retryInterval, timeout)
}

// WaitForStatefulSet checks and waits for a given statefulset to be in ready.
func WaitForStatefulSet(t *testing.T, kubeclient kubernetes.Interface, namespace string, name string, replicas int, retryInterval time.Duration, timeout time.Duration) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		statefulset, err := kubeclient.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{IncludeUninitialized: true})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s statefulset\n", name)
				return false, nil
			}
			return false, err
		}

		if int(statefulset.Status.ReadyReplicas) == replicas {
			return true, nil
		}

		t.Logf("Waiting for ready status of %s statefulset (%d)\n", name, statefulset.Status.ReadyReplicas)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("StatefulSet available (%d/%d)\n", replicas, replicas)
	return nil
}

func GetInsightsClientPod(namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "insights-client",
			Namespace: namespace,
			Labels: map[string]string{
				"app":   "demo",
				"group": "nuodb",
			},
		},
		Spec: corev1.PodSpec{
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "nuodb.com/zone",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
					PreferredDuringSchedulingIgnoredDuringExecution: nil,
				},
			},
			Volumes: []corev1.Volume{
				{"log-volume", corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},},
				{"config-insights", corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "insights-configmap"},
					},
				}},
				{"nuoinsights", corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "insights-configmap"},
					},},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "insights",
					Image:           "nuodb/nuodb-ce:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"/opt/nuodb/etc/python/x86_64-linux/bin/python2.7",
					},
					Args: []string{
						"/opt/nuodb/etc/nuoca/src/nuoca.py",
						"--collection-interval",
						"10",
						"-o",
						"sub_id=INSIGHTS",
						"/etc/nuodb/nuoca.local.yml",
					},
					Env: []corev1.EnvVar{
						{Name: "NUOCMD_API_SERVER", Value: "https://domain:8888", ValueFrom: nil},
						{Name: "PYTHONWARNINGS", Value: "ignore:Unverified HTTPS request", ValueFrom: nil},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "log-volume", MountPath: "/var/log/nuodb"},
						{Name: "config-insights", MountPath: "/etc/nuodb/nuoca.local.yml", SubPath: "nuoca.local.yml"},
					},
				},
			},
			},
		}
	}

func CreateInsightsPods(t *testing.T, ctx *framework.TestCtx, insightsPod *corev1.Pod) error{
	f := framework.Global
	err := f.Client.Create(goctx.TODO(), insightsPod, &framework.CleanupOptions{TestContext: ctx, Timeout: CleanupTimeout, RetryInterval: CleanupRetryInterval})
	assert.NilError(t, err)
	return nil
}

//ExecCommand executes arbitrary command inside the pod
func ExecCommand(f *framework.Framework, namespace string, podName string, containerName string, command []string) (string, error) {

	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)

	print(command)
	pod, err := f.KubeClient.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("could not get pod info: %v", err)
	}

	targetContainer := -1
	for i, cr := range pod.Spec.Containers {
		if cr.Name == containerName {
			targetContainer = i
			break
		}
	}

	if targetContainer < 0 {
		return "", fmt.Errorf("could not find %s container to exec to", podName)
	}

	req := f.KubeClient.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	config1 := f.KubeConfig
	exec, err := remotecommand.NewSPDYExecutor(config1, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to init executor: %v", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
	})

	if err != nil {
		return "", fmt.Errorf("could not execute: %v", err)
	}

	if execErr.Len() > 0 {
		return "", fmt.Errorf("stderr: %v", execErr.String())
	}

	return execOut.String(), nil
}
