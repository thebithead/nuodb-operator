package e2e

import (
	goctx "context"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	"gotest.tools/assert"
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
		smMemory           = "500Mi"
		smCount           int32 = 1
		smCpu              = "100m"
		smStorageSize           = "20G"
		smStorageClass          = "local-disk"
		engineOptions           = ""
		teCount           int32 = 1
		teMemory           = "500Mi"
		teCpu              = "100m"
		apiServer               = "https://domain:8888"
		container               = "nuodb/nuodb-ce:latest"
	)

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}

	nuodbAdminSpec := operator.NuodbAdminSpec{
		AdminCount:        adminCount,
		AdminStorageClass: adminStorageClass,
		AdminStorageSize:  adminStorageSize,
		StorageMode:       storageMode,
		InsightsEnabled:   false,
		ApiServer:         apiServer,
		Container:         container,
	}

	nuodbSpec := operator.NuodbSpec{
		StorageMode:       storageMode,
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
		Container:         container,
	}

	exampleNuodbAdmin := testutil.NewNuodbAdmin(namespace, nuodbAdminSpec)
	testutil.SetupOperator(t,ctx)
	err = testutil.DeployNuodbAdmin(t, ctx, exampleNuodbAdmin )
	assert.NilError(t, err)

	exampleNuodb := testutil.NewNuodbDatabase(namespace, nuodbSpec)
	err = testutil.DeployNuodb(t, ctx, exampleNuodb )
	assert.NilError(t, err)

	f := framework.Global

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
	assert.NilError(t, err)

	//Scale up TE
	exampleNuodb.Spec.TeCount = 2
	err = f.Client.Update(goctx.TODO(), exampleNuodb)
	assert.NilError(t, err)

	// wait for te to reach 2 replicas
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "nuodb-te", 2, testutil.RetryInterval, testutil.Timeout)
	assert.NilError(t, err)
	t.Log("Te scaled up successfully")

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
	assert.NilError(t, err)

	//Scale down TE
	exampleNuodb.Spec.TeCount = 1
	err = f.Client.Update(goctx.TODO(), exampleNuodb)
	assert.NilError(t, err)

	// wait for te to reach 1 replicas
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "nuodb-te", 1, testutil.RetryInterval, testutil.Timeout)
	assert.NilError(t, err)
	t.Log("Te scaled down successfully")


	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuoadmin", Namespace: namespace}, exampleNuodbAdmin)
	assert.NilError(t, err)

	//Scale down Admin
	exampleNuodbAdmin.Spec.AdminCount = 0
	err = f.Client.Update(goctx.TODO(), exampleNuodbAdmin)
	assert.NilError(t, err)

	err = testutil.WaitForStatefulSet(t, f.KubeClient, namespace, "admin",0, testutil.RetryInterval, testutil.Timeout*2)
	assert.NilError(t, err)
	t.Log("Admin scaled down to 0 successfully")

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuoadmin", Namespace: namespace}, exampleNuodbAdmin)
	assert.NilError(t, err)

	//Scale Up Admin
	exampleNuodbAdmin.Spec.AdminCount = 1
	err = f.Client.Update(goctx.TODO(), exampleNuodbAdmin)
	assert.NilError(t, err)

	err = testutil.WaitForStatefulSet(t, f.KubeClient, namespace, "admin",1, testutil.RetryInterval, testutil.Timeout*2)
	assert.NilError(t, err)
	t.Log("Admin scaled up to 1 successfully")

	testutil.VerifyAdminState(t, f, namespace, "admin-0", "admin")

}


