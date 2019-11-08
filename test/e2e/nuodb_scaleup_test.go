package e2e

import (
	goctx "context"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"testing"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	operator "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	testutil "nuodb/nuodb-operator/test/e2e/util"

	"k8s.io/apimachinery/pkg/types"
)

func TestNuodbScale(t *testing.T) {
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
		smMemory           = "500m"
		smCount           int32 = 1
		smCpu              = "100m"
		smStorageSize           = "20G"
		smStorageClass          = "local-disk"
		engineOptions           = ""
		teCount           int32 = 1
		teMemory           = "100m"
		teCpu              = "100m"
		apiServer               = "https://domain:8888"
		container               = "nuodb/nuodb-ce:latest"
	)

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}

	clusterSpec:= operator.NuodbSpec{
		StorageMode:       storageMode,
		InsightsEnabled:   true,
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

	//Scale up TE
	exampleNuodb.Spec.TeCount = 2
	err = f.Client.Update(goctx.TODO(), exampleNuodb)
	if err != nil {
		t.Fatal(err)
	}

	// wait for te to reach 2 replicas
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "te", 2, testutil.RetryInterval, testutil.Timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Te scaled up successfully")

	//Scale down TE
	exampleNuodb.Spec.TeCount = 1
	err = f.Client.Update(goctx.TODO(), exampleNuodb)
	if err != nil {
		t.Fatal(err)
	}

	// wait for te to reach 1 replicas
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "te", 1, testutil.RetryInterval, testutil.Timeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Te scaled down successfully")

	//Scale down Admin
	exampleNuodb.Spec.AdminCount = 0
	err = f.Client.Update(goctx.TODO(), exampleNuodb)
	if err != nil {
		t.Fatal(err)
	}

	err = testutil.WaitForStatefulSet(t, f.KubeClient, namespace, "admin",0, testutil.RetryInterval, testutil.Timeout*2)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Admin scaled down to 0 successfully")

	//Scale Up Admin
	exampleNuodb.Spec.AdminCount = 1
	err = f.Client.Update(goctx.TODO(), exampleNuodb)
	if err != nil {
		t.Fatal(err)
	}

	err = testutil.WaitForStatefulSet(t, f.KubeClient, namespace, "admin",1, testutil.RetryInterval, testutil.Timeout*2)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Admin scaled up to 1 successfully")

	testutil.VerifyAdminState(t, f, namespace, "admin-0", "admin")

}


