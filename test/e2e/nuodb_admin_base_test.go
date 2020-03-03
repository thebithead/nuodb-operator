package e2e

import (
	goctx "context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	operator "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	testutil "nuodb/nuodb-operator/test/e2e/util"

	"k8s.io/apimachinery/pkg/types"
)


func verifyLoadBalancer(t *testing.T, f *framework.Framework, namespaceName string,balancerName string) {
	var service = &corev1.Service{}
	err := f.Client.Get(goctx.TODO(), client.ObjectKey{Namespace: namespaceName, Name: balancerName}, service)
	if err!=nil{
		t.Fatalf("Couldn't get service %+v", err)
	}
	assert.Equal(t, service.Name, balancerName)
}

func verifyPodKill(t *testing.T, f *framework.Framework, namespaceName string, podName string, expectedName string, expReplicas int) {
	testutil.KillAdminPod(t, f, namespaceName, podName)
	testutil.AwaitNrReplicasScheduled(t, f, namespaceName, expectedName, expReplicas)
	testutil.AwaitAdminPodUp(t, f, namespaceName, podName, 100 * time.Second)
}

func verifyKillProcess(t *testing.T, f *framework.Framework, namespaceName string, podName string, containerName string, nrReplicasExpected int) {
	testutil.KillAdminProcess(t, f, namespaceName, podName)
	testutil.AwaitNrReplicasScheduled(t, f, namespaceName, containerName, nrReplicasExpected)
	testutil.AwaitAdminPodUp(t, f, namespaceName, podName, 100 * time.Second)
}

func verifyAdminService(t *testing.T, f *framework.Framework, namespaceName string, podName string) {
	serviceName := "domain"
	var service = &corev1.Service{}
	err := f.Client.Get(goctx.TODO(), client.ObjectKey{Namespace: namespaceName, Name: serviceName}, service)
	if err!=nil{
		t.Fatalf("Couldn't get service %+v", err)
	}

	testutil.PingService(t, f, namespaceName, serviceName, podName)
}


func TestNuodbAdmin(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	var (
		namespace               = "nuodb"
		storageMode             = "ephemeral"
		adminCount        int32 = 1
		adminStorageSize        = "5G"
		adminStorageClass       = "local-disk"
		apiServer               = "https://domain:8888"
		container               = "nuodb/nuodb-ce:latest"
	)

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}

	adminSpec := operator.NuodbAdminSpec{
		StorageMode:       storageMode,
		InsightsEnabled:   true,
		AdminCount:        adminCount,
		AdminStorageSize:  adminStorageSize,
		AdminStorageClass: adminStorageClass,
		ApiServer:         apiServer,
		Container:         container,
	}

	exampleNuodb := testutil.NewNuodbAdmin(namespace, adminSpec)
	testutil.SetupOperator(t,ctx)
	err = testutil.DeployNuodbAdmin(t, ctx, exampleNuodb )
	if err != nil {
		t.Fatal(err)
	}

	f := framework.Global

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuoadmin", Namespace: namespace}, exampleNuodb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("verifyAdminState", func(t *testing.T) { testutil.VerifyAdminState(t, f, namespace, "admin-0", "admin") })
	t.Run("verifyLoadBalancer", func(t *testing.T) { verifyLoadBalancer(t, f, namespace, "admin") })
	t.Run("verifyPodKill", func(t *testing.T) { verifyPodKill(t, f, namespace, "admin-0", "admin-0", 1) })
	t.Run("verifyProcessKill", func(t *testing.T) { verifyKillProcess(t, f, namespace, "admin-0", "admin-0", 1) })
	t.Run("verifyAdminService", func(t *testing.T) { verifyAdminService(t, f, namespace, "admin-0") })

}

