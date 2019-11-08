package e2e

import (
	 "context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	operator "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	testutil "nuodb/nuodb-operator/test/e2e/util"

	"k8s.io/apimachinery/pkg/types"
)

var (
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

func verifySecret(t *testing.T, f *framework.Framework, namespaceName string) {
	var secret = &corev1.Secret{}
	err := f.Client.Get(context.TODO(), client.ObjectKey{Namespace: namespaceName, Name: dbName + ".nuodb.com"}, secret)

	assert.NilError(t, err)

	_, ok := secret.Data["database-name"]
	assert.Check(t, ok)

	_, ok = secret.Data["database-password"]
	assert.Check(t, ok)

	_, ok = secret.Data["database-username"]
	assert.Check(t, ok)
}

func TestNuodbDatabase(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()


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

	err = f.Client.Get(context.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("verifySecret", func(t *testing.T) { verifySecret(t, f, namespace) })
}
