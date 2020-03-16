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
		apiServer               = "https://domain:8888"
		container               = "nuodb/nuodb-ce:latest"
	)

	namespace, err := ctx.GetNamespace()
	assert.NilError(t, err)

	nuodbAdminSpec := operator.NuodbAdminSpec{
		AdminCount:        adminCount,
		AdminStorageClass: adminStorageClass,
		AdminStorageSize:  adminStorageSize,
		StorageMode:       storageMode,
		InsightsEnabled:   false,
		ApiServer:         apiServer,
		Container:         container,
	}

	exampleNuodbAdmin := testutil.NewNuodbAdmin(namespace, nuodbAdminSpec)
	testutil.SetupOperator(t, ctx)
	testutil.DeployNuodbAdmin(t, ctx, exampleNuodbAdmin)

	f := framework.Global

	t.Run("adminTest", func(t *testing.T) {
		t.Run("scaleDownAdmin", func(t *testing.T) {
			defer func() {
				err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuoadmin", Namespace: namespace}, exampleNuodbAdmin)
				assert.NilError(t, err)

				exampleNuodbAdmin.Spec.AdminCount = adminCount
				err = f.Client.Update(goctx.TODO(), exampleNuodbAdmin)
				assert.NilError(t, err)

				err = testutil.WaitForStatefulSet(t, f.KubeClient, namespace, "admin", 1, testutil.RetryInterval, testutil.Timeout*2)
				assert.NilError(t, err)

				testutil.VerifyAdminState(t, f, namespace, "admin-0", "admin")
			}()

			err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuoadmin", Namespace: namespace}, exampleNuodbAdmin)
			assert.NilError(t, err)

			exampleNuodbAdmin.Spec.AdminCount = 0
			err = f.Client.Update(goctx.TODO(), exampleNuodbAdmin)
			assert.NilError(t, err)

			err = testutil.WaitForStatefulSet(t, f.KubeClient, namespace, "admin", 0, testutil.RetryInterval, testutil.Timeout*2)
			assert.NilError(t, err)
		})
	})

	t.Run("databaseTest", func(t *testing.T) {
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

		t.Run("scaleUpTE", func(t *testing.T) {
			defer func() {
				err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
				assert.NilError(t, err)

				exampleNuodb.Spec.TeCount = teCount
				err = f.Client.Update(goctx.TODO(), exampleNuodb)
				assert.NilError(t, err)

				err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "nuodb-te", 1, testutil.RetryInterval, testutil.Timeout)
				assert.NilError(t, err)
			}()

			err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
			assert.NilError(t, err)

			exampleNuodb.Spec.TeCount = 2
			err = f.Client.Update(goctx.TODO(), exampleNuodb)
			assert.NilError(t, err)

			// wait for te to reach 2 replicas
			err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "nuodb-te", 2, testutil.RetryInterval, testutil.Timeout)
			assert.NilError(t, err)
		})

		t.Run("scaleDownTE", func(t *testing.T) {
			defer func() {
				err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
				assert.NilError(t, err)

				exampleNuodb.Spec.TeCount = teCount
				err = f.Client.Update(goctx.TODO(), exampleNuodb)
				assert.NilError(t, err)

				err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "nuodb-te", 1, testutil.RetryInterval, testutil.Timeout)
				assert.NilError(t, err)
			}()

			err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: "nuodb", Namespace: namespace}, exampleNuodb)
			assert.NilError(t, err)

			exampleNuodb.Spec.TeCount = 0
			err = f.Client.Update(goctx.TODO(), exampleNuodb)
			assert.NilError(t, err)

			// wait for te to reach 2 replicas
			err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "nuodb-te", 0, testutil.RetryInterval, testutil.Timeout)
			assert.NilError(t, err)
		})
	})

}