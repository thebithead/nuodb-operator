package nuodbycsbwl

import (
	"context"
	"fmt"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	nuodbv2alpha1 "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"testing"
)

var cl = fake.NewFakeClient()
var namespace = "nuodb"
var s = scheme.Scheme
var r = &ReconcileNuodbYcsbWl{}
var req = reconcile.Request{}

func init() {
	var (
		name                    = "nuodb-ycsb"
		namespace               = "nuodb"
		dbName 					= "test1"
		ycsbWorkloadCount int32 =  1
		ycsbLoadName			=  "ycsb-load"
		ycsbWorkload 			=  "b"
		ycsbLbPolicy 			=  ""
		ycsbNoOfProcesses int32 =  2
		ycsbNoOfRows      int32 =  10000
		ycsbNoOfIterations 		  int32 =  0
		ycsbOpsPerIteration  int32 	=  10000
		ycsbMaxDelay 		 int32 	=  240000
		ycsbDbSchema 			=  "User1"
	)

	logf.SetLogger(logf.ZapLogger(true))
	nuodbycsbw := &nuodbv2alpha1.NuodbYcsbWl{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: nuodbv2alpha1.NuodbYcsbWlSpec{
			DbName:  dbName,
			YcsbWorkloadCount:  ycsbWorkloadCount,
			YcsbLoadName:  ycsbLoadName,
			YcsbWorkload:  ycsbWorkload,
			YcsbLbPolicy:  ycsbLbPolicy,
			YcsbNoOfProcesses:  ycsbNoOfProcesses,
			YcsbNoOfRows:  ycsbNoOfRows,
			YcsbNoOfIterations:  ycsbNoOfIterations,
			YcsbOpsPerIteration:  ycsbOpsPerIteration,
			YcsbMaxDelay:  ycsbMaxDelay,
			YcsbDbSchema:  ycsbDbSchema,
		},
	}
	// Objects to track in the fake client.
	objs := []runtime.Object{
		nuodbycsbw,
	}

	// Register operator types with the runtime scheme.
	s = scheme.Scheme

	s.AddKnownTypes(nuodbv2alpha1.SchemeGroupVersion, nuodbycsbw)
	// Create a fake client to mock API calls.
	cl = fake.NewFakeClient(objs...)

	r = &ReconcileNuodbYcsbWl{client: cl, scheme: s}

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	_, err := reconcileNuodbYcsbWlInternal(r, req)
	if err != nil {
		fmt.Printf(" Reconcile Failed with error  %v\n", err)
	}

}

func Test_createNuodbYcsbWlReplicationController(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbYcsbWlInstance(r, req)
	assert.NilError(t, err)

	template :=`apiVersion: v1
kind: ReplicationController
metadata:
  name: test-replicationcontroller
spec:
  replicas: 3
  selector:
    app: nginx
  template:
    metadata:
      name: nginx
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80`

	t.Run("Test With new replicationcontroller", func(t *testing.T) {
		_, err = createNuodbYcsbWlReplicationController(cl, s, req, instance, NuoYcsbWlResource{
			template: template,
			name: "test-replicationcontroller",
		})
		assert.NilError(t, err)
		var replicationController = &corev1.ReplicationController{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-replicationcontroller", Namespace: "nuodb"}, replicationController)
		assert.NilError(t, err)

		assert.Equal(t, replicationController.ObjectMeta.Name, "test-replicationcontroller")
	})

	t.Run("Test With error replicationcontroller", func(t *testing.T) {
		_, err = createNuodbYcsbWlReplicationController(cl, s, req, instance, NuoYcsbWlResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})

}

func Test_reconcileNuodbYcsbWlReplicationController(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbYcsbWlInstance(r, req)
	assert.NilError(t, err)

	template :=`apiVersion: v1
kind: ReplicationController
metadata:
  name: test-replicationcontroller
spec:
  replicas: 3
  selector:
    app: nginx
  template:
    metadata:
      name: nginx
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
        ports:
        - containerPort: 80`


	t.Run("Test With new replicationcontroller", func(t *testing.T) {
		_,_, err = reconcileNuodbYcsbWlReplicationController(cl, s, req, instance, NuoYcsbWlResource{
			template: template,
			name: "test-replicationcontroller",
		}, namespace)
		assert.NilError(t, err)

		var replicationController = &corev1.ReplicationController{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-replicationcontroller", Namespace: "nuodb"}, replicationController)
		assert.NilError(t, err)

		repSize := replicationController.Spec.Replicas
		assert.Equal(t, *repSize, int32(3))
	})

	t.Run("Existing-replicationcontroller", func(t *testing.T) {
		_,_, err = reconcileNuodbYcsbWlReplicationController(cl, s, req, instance, NuoYcsbWlResource{
			template: template,
			name: "ycsb-load",
		}, namespace)
		assert.NilError(t, err)

		var replicationController = &corev1.ReplicationController{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "ycsb-load", Namespace: "nuodb"}, replicationController)
		assert.NilError(t, err)

		repSize := replicationController.Spec.Replicas
		assert.Equal(t, *repSize, int32(1))
	})

	t.Run("Test update replicationcontroller", func(t *testing.T) {
		instance.Spec.YcsbWorkloadCount = 3
		defer func() {
			instance.Spec.YcsbWorkloadCount = 1
		}()

		_,_, err = reconcileNuodbYcsbWlReplicationController(cl, s, req, instance, NuoYcsbWlResource{
			template: template,
			name: "ycsb-load",
		}, namespace)
		assert.NilError(t, err)

		var replicationController = &corev1.ReplicationController{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "ycsb-load", Namespace: "nuodb"}, replicationController)
		assert.NilError(t, err)

		repSize := replicationController.Spec.Replicas
		assert.Equal(t, *repSize, int32(3))
	})

}