package e2e

import (
	goctx "context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
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
	smMemory                = "500Mi"
	smCount           int32 = 1
	smCpu                   = "100m"
	smStorageSize           = "20G"
	smStorageClass          = "local-disk"
	engineOptions           = ""
	teCount           int32 = 1
	teMemory                = "500Mi"
	teCpu                   = "100m"
	apiServer               = "https://domain:8888"
	container               = "nuodb/nuodb-ce:latest"
)

func verifySecret(t *testing.T, f *framework.Framework, namespaceName string) {
	var secret = &corev1.Secret{}
	err := f.Client.Get(goctx.TODO(), client.ObjectKey{Namespace: namespaceName, Name: dbName + ".nuodb.com"}, secret)
	assert.NilError(t, err)

	_, ok := secret.Data["database-name"]
	assert.Check(t, ok)

	_, ok = secret.Data["database-password"]
	assert.Check(t, ok)

	_, ok = secret.Data["database-username"]
	assert.Check(t, ok)
}

func verifyNuoSQL(t *testing.T, f *framework.Framework, namespace string, adminPod string, databaseName string) {

	cmd := []string{"bash", "-c", fmt.Sprintf("echo \"select * from system.nodes;\" | nuosql %s@localhost --user dba --password secret", databaseName)}

	output, err := testutil.ExecCommand(f, namespace, adminPod, "admin", cmd)

	assert.NilError(t, err, output)

	assert.Check(t, strings.Contains(output, "Storage"))
	assert.Check(t, strings.Contains(output, "Transaction"))
}


func TestNuodbDatabase(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	namespace, err := ctx.GetNamespace()
	assert.NilError(t, err)

	adminSpec := operator.NuodbAdminSpec{
		StorageMode:       storageMode,
		InsightsEnabled:   true,
		AdminCount:        adminCount,
		AdminStorageSize:  adminStorageSize,
		AdminStorageClass: adminStorageClass,
		ApiServer:         apiServer,
		Container:         container,
	}

	exampleNuodbAdmin := testutil.NewNuodbAdmin(namespace, adminSpec)
	testutil.SetupOperator(t, ctx)
	testutil.DeployNuodbAdmin(t, ctx, exampleNuodbAdmin)

	f := framework.Global

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuoadmin", Namespace: namespace}, exampleNuodbAdmin)
	assert.NilError(t, err)

	nuodbSpec := operator.NuodbSpec{
		StorageMode:    storageMode,
		DbName:         dbName,
		DbUser:         dbUser,
		DbPassword:     dbPassword,
		SmMemory:       smMemory,
		SmCount:        smCount,
		SmCpu:          smCpu,
		SmStorageSize:  smStorageSize,
		SmStorageClass: smStorageClass,
		EngineOptions:  engineOptions,
		TeCount:        teCount,
		TeMemory:       teMemory,
		TeCpu:          teCpu,
		Container:      container,
	}

	exampleNuodb := testutil.NewNuodbDatabase(namespace, nuodbSpec)
	testutil.DeployNuodb(t, ctx, exampleNuodb)

	err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
	assert.NilError(t, err)

	t.Run("verifySecret", func(t *testing.T) { verifySecret(t, f, namespace) })
	t.Run("verifyNuoSQL", func(t *testing.T) { verifyNuoSQL(t, f, namespace, "admin-0", dbName) })
}
