package nuodbycsbwl

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	nuodbv2alpha1 "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	"reflect"
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
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
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

	var tests = []struct {
		name string
		in  NuoYcsbWlResource
		out string
	}{
		{"Test With new replicationcontroller",NuoYcsbWlResource{
			template: template,
			name: "test-replicationcontroller",
		},"test-replicationcontroller"},
		{"Test With Error replicationcontroller",NuoYcsbWlResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbYcsbWlReplicationController(cl, s, req, instance, tt.in)
			var replicationController = &corev1.ReplicationController{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, replicationController)
			if err != nil {
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test replicationController not found : (%v)", err)
			}

			if !reflect.DeepEqual(replicationController.ObjectMeta.Name,tt.out) {
				t.Errorf("Created replicationController doesnt has same name (%v)", replicationController.Name)
			}
		})
	}

}

func Test_reconcileNuodbYcsbWlReplicationController(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbYcsbWlInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
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

	var tests = []struct {
		name string
		in  NuoYcsbWlResource
		out int32
	}{
		{"Test With new replicationcontroller",NuoYcsbWlResource{
			template: template,
			name: "test-replicationcontroller",
		},3},
		{"Existing-replicationcontroller",NuoYcsbWlResource{
			template: template,
			name: "ycsb-load",
		},1},
		{"Update-replicationcontroller",NuoYcsbWlResource{
			template: template,
			name: "ycsb-load",
		},3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name=="Update-replicationcontroller"{
				instance.Spec.YcsbWorkloadCount = 3
			}
			_,_, err = reconcileNuodbYcsbWlReplicationController(cl, s, req, instance, tt.in, namespace)
			var replicationController = &corev1.ReplicationController{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, replicationController)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test replicationController not found : (%v)", err)
			}
			repSize := replicationController.Spec.Replicas

			if *repSize!=tt.out {
				t.Errorf("Created replicationController doesnt has same Count (%d)", repSize)
			}
		})
	}
}