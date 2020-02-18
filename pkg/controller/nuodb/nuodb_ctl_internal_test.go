package nuodb

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	nuodbv2alpha1 "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	"nuodb/nuodb-operator/pkg/utils"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"testing"
)

var cl = fake.NewFakeClient()
var namespace = "nuodb"
var replicas int32 = 1
var s = scheme.Scheme
var r = &ReconcileNuodb{client: cl, scheme: s}
var req = reconcile.Request{}

func init() {
	var (
		name                    = "nuodb-operator"
		namespace               = "nuodb"
		storageMode             = "ephemeral"
		dbName                  = "test1"
		dbUser                  = "dba"
		dbPassword              = "secret"
		smMemory                = "2Gi"
		smCount           int32 = 1
		smCpu                   = "1"
		smStorageSize           = "20G"
		smStorageClass          = "local-disk"
		engineOptions           = ""
		teCount           int32 = 1
		teMemory                = "2Gi"
		teCpu                   = "1"
		container               = "nuodb/nuodb-ce:latest"
	)

	logf.SetLogger(logf.ZapLogger(true))
	nuodb := &nuodbv2alpha1.Nuodb{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: nuodbv2alpha1.NuodbSpec{
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
		},
		Status: nuodbv2alpha1.NuodbStatus{
		ControllerVersion: utils.NuodbOperatorVersion,
		Phase:             utils.NuodbPendingPhase,
		SmReadyCount:      0,
		TeReadyCount:      0,
		SmHealth:          utils.NuodbUnknownHealth,
		TeHealth:          utils.NuodbUnknownHealth,
		DatabaseHealth:    utils.NuodbUnknownHealth,
	},
	}

	nl := [1] nuodbv2alpha1.Nuodb {*nuodb}
	nuodbList := &nuodbv2alpha1.NuodbList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "nuodbs.nuodb.com/v2alpha1",
			Kind:       "NuodbList",
		},
		ListMeta: metav1.ListMeta{},
		Items:    nl[1:1],
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{
		nuodb,
		nuodbList,
	}

	// Register operator types with the runtime scheme.
	s = scheme.Scheme

	s.AddKnownTypes(nuodbv2alpha1.SchemeGroupVersion, nuodb)
	s.AddKnownTypes(nuodbv2alpha1.SchemeGroupVersion, nuodbList)
	// Create a fake client to mock API calls.
	cl = fake.NewFakeClient(objs...)

	r = &ReconcileNuodb{client: cl, scheme: s}

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	_, err := reconcileNuodbInternal(r, req)
	if err != nil {
		fmt.Printf(" Reconcile Failed with error  %v\n", err)
	}
}

func Test_createNuodbService(t *testing.T) {
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("test servie error : (%v)", err)
	}
	var service = &corev1.Service{}

	template := `apiVersion: v1
kind: Service
metadata:
  annotations:
    description: "Service for redirect."
  labels:
    group: nuodb
    subgroup: monitoring
    app: insights
  name: test-service
spec:
  ports:
  - { name: 8080-tcp,   port: 8080,   protocol: TCP,  targetPort: 8080  }
  selector:
    app: test
    group: nuodb
  sessionAffinity: None
  type: LoadBalancer
status:
  loadBalancer: {}`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Service",NuoResource{
			template: template,
			name: "test-service",
		},"test"},
		{"Test With Error Service",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbService(cl, s, req, instance, tt.in)
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, service)
			if err != nil {
				//Case where the given pod is not found by the reconcile funciton
				//and new pod cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test Service not found : (%v)", err)
			}

			if!reflect.DeepEqual(service.Spec.Selector,map[string]string{"app": tt.out,"group": "nuodb"}){
				t.Errorf("reconcileService(%s) got %v, want %v", tt.in, service, tt.out)
			}
		})
	}
}

func Test_reconcileNuodbService(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("test servie error : (%v)", err)
	}
	var service = &corev1.Service{}

	template := `apiVersion: v1
kind: Service
metadata:
  annotations:
    description: "Service for redirect."
  labels:
    group: nuodb
    subgroup: monitoring
    app: insights
  name: test-service
spec:
  ports:
  - { name: 8080-tcp,   port: 8080,   protocol: TCP,  targetPort: 8080  }
  selector:
    app: test
    group: nuodb
  sessionAffinity: None
  type: LoadBalancer
status:
  loadBalancer: {}`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Service",NuoResource{
			template: template,
			name: "test-service",
		},"test"},
		{"Test With Error Service",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_,_, err = reconcileNuodbService(cl, s, req, instance, tt.in, namespace)
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, service)
			if err != nil {
				//Case where the given pod is not found by the reconcile funciton
				//and new pod cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test Service not found : (%v)", err)
			}

			if!reflect.DeepEqual(service.Spec.Selector,map[string]string{"app": tt.out,"group": "nuodb"}){
				t.Errorf("reconcileService(%s) got %v, want %v", tt.in, service, tt.out)
			}
		})
	}
}

func Test_getDeployment(t *testing.T)  {
	var deployment *appsv1.Deployment = nil
	deployment, err := utils.GetDeployment(cl, namespace, "nuodb-operator-te")
	if err != nil {
		t.Fatalf("Get Deployment error : (%v)", err)
	}

	dctesize := deployment.Spec.Replicas
	if *dctesize != int32(1) {
		t.Errorf("dep size (%d) is not the expected size (%d)", dctesize, replicas)
	}
}

func Test_createNuodbDeployment(t *testing.T) {
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.7.9
        ports:
        - containerPort: 80`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Deployment",NuoResource{
			template: template,
			name: "test-deployment",
		},"3"},
		{"Test With Error Deployment",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbDeployment(cl, s, req, instance, tt.in)
			var dcte = &appsv1.Deployment{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, dcte)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test Deployment not found : (%v)", err)
			}

			dctesize := dcte.Spec.Replicas
			if *dctesize != int32(3) {
				t.Errorf("dep size (%d) is not the expected size (%d)", dctesize, replicas)
			}
		})
	}
}

func Test_reconcileNuodbDeployment(t *testing.T) {
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-new
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.7.9
        ports:
        - containerPort: 80`

	var tests = []struct {
		name string
		in  NuoResource
		out int32
	}{
		{"Test With new Deployment",NuoResource{
			template: template,
			name: "test-deployment-new",
		},1},
		{"Existing-Deployment",NuoResource{
			template: template,
			name: "te",
		},1},
		{"Update-Count",NuoResource{
			name: "te",
		} ,3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name=="Update-Count"{
				instance.Spec.TeCount=3
			}
			_,_, err = reconcileNuodbDeployment(cl, s, req, instance, tt.in, namespace)
			var dcte = &appsv1.Deployment{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, dcte)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test Deployment not found : (%v)", err)
			}

			dctesize := dcte.Spec.Replicas
			if *dctesize != tt.out {
				t.Errorf("dep size (%d) is not the expected size (%d)", dctesize, tt.out)
			}
		})
	}

}

func Test_createNuodbStatefulSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-statefulset
spec:
  selector:
    matchLabels:
      app: nginx # has to match .spec.template.metadata.labels
  serviceName: "nginx"
  replicas: 2 # by default is 1
  template:
    metadata:
      labels:
        app: nginx # has to match .spec.selector.matchLabels
    spec:
      terminationGracePeriodSeconds: 10
      containers:
      - name: nginx
        image: k8s.gcr.io/nginx-slim:0.8
        ports:
        - containerPort: 80
          name: web
        volumeMounts:
        - name: www
          mountPath: /usr/share/nginx/html
  volumeClaimTemplates:
  - metadata:
      name: www
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: "my-storage-class"
      resources:
        requests:
          storage: 1Gi`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new StatefulSet",NuoResource{
			template: template,
			name: "test-statefulset",
		},"3"},
		{"Test With Error StatefulSet",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbStatefulSet(cl, s, req, instance, tt.in)
			var statefulSet = &appsv1.StatefulSet{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, statefulSet)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test StatefulSet not found : (%v)", err)
			}

			stsSize := statefulSet.Spec.Replicas
			if *stsSize != int32(2) {
				t.Errorf("StatefulSet Size (%d) is not the expected size (%d)", stsSize, 2)
			}
		})
	}
}

func Test_reconcileNuodbStatefulSet(t *testing.T){

	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}

	template :=`apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-statefulset
spec:
  selector:
    matchLabels:
      app: nginx # has to match .spec.template.metadata.labels
  serviceName: "nginx"
  replicas: 2 # by default is 1
  template:
    metadata:
      labels:
        app: nginx # has to match .spec.selector.matchLabels
    spec:
      terminationGracePeriodSeconds: 10
      containers:
      - name: nginx
        image: k8s.gcr.io/nginx-slim:0.8
        ports:
        - containerPort: 80
          name: web
        volumeMounts:
        - name: www
          mountPath: /usr/share/nginx/html
  volumeClaimTemplates:
  - metadata:
      name: www
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: "my-storage-class"
      resources:
        requests:
          storage: 1Gi`

	var tests = []struct {
		name string
		in  NuoResource
		out int32
	}{
		{"Test With new Statefulset",NuoResource{
			template: template,
			name: "test-statefulset",
		},2},
		{"Existing-Statefulset",NuoResource{
			template: template,
			name: "sm",
		},1},
		{"Update-Count-sm",NuoResource{
			name: "sm",
		} ,3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name=="Update-Count-sm"{
				instance.Spec.SmCount=3
			}
			_,_, err = reconcileNuodbStatefulSet(cl, s, req, instance, tt.in, namespace)
			var statefulSet = &appsv1.StatefulSet{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, statefulSet)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test Deployment not found : (%v)", err)
			}

			stsSize := statefulSet.Spec.Replicas
			if *stsSize != tt.out {
				t.Errorf("sts size (%d) is not the expected size (%d)", stsSize, tt.out)
			}
		})
	}

}

func Test_createNuodbConfigMap(t *testing.T){

	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`kind: ConfigMap 
apiVersion: v1 
metadata:
  name: test-configmap 
data:
  database: nuodb`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Configmap",NuoResource{
			template: template,
			name: "test-configmap",
		},"nuodb"},
		{"Test With Error Configmap",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbConfigMap(cl, s, req, instance, tt.in)
			var configMap = &corev1.ConfigMap{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, configMap)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test Configmap not found : (%v)", err)
			}

			if !reflect.DeepEqual(configMap.Data,map[string]string{"database": tt.out}) {
				t.Errorf("Created config map doesnt has same data (%v)", configMap.Data)
			}
		})
	}

}

func Test_reconcileNuodbConfigMap(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`kind: ConfigMap 
apiVersion: v1 
metadata:
  name: test-configmap 
data:
  database: nuodb`


	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new ConfigMap",NuoResource{
			template: template,
			name: "test-configmap",
		},"nuodb"},
		{"Existing-ConfigMap",NuoResource{
			template: template,
			name: "nuodb-insights",
		},"nuodb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_,_, err = reconcileNuodbConfigMap(cl, s, req, instance, tt.in, namespace)
			var configMap = &corev1.ConfigMap{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, configMap)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test Deployment not found : (%v)", err)
			}

			if !reflect.DeepEqual(configMap.Data,map[string]string{"database": tt.out}) {
				t.Errorf("Created config map doesnt has same data (%v)", configMap.Data)
			}
		})
	}
}

func Test_getSecret(t *testing.T){
	var secret = &corev1.Secret{}
	secret, err := utils.GetSecret(cl, namespace,"")
	if err != nil{
		t.Fatalf("Get secret error : (%v)", err)
	}

	if secret.ObjectMeta.Name != "test1.nuodb.com"{
		t.Errorf("secret  name (%v) is not as expected ", secret.ObjectMeta.Name)
	}
}

func Test_createNuodbSecret(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: Opaque
data:
  test: test`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Secret",NuoResource{
			template: template,
			name: "test-secret",
		},"test-secret"},
		{"Test With Error secret",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbSecret(cl, s, req, instance, tt.in)
			var secret = &corev1.Secret{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, secret)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test Secret not found : (%v)", err)
			}

			if !reflect.DeepEqual(secret.ObjectMeta.Name,tt.out) {
				t.Errorf("Created secret map doesnt has same name (%v)", secret.Name)
			}
		})
	}

}

func Test_reconcileNuodbSecret(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: Opaque
data:
  test: test`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Secret",NuoResource{
			template: template,
			name: "test-secret",
		},"test-secret"},
		{"Existing-secret",NuoResource{
			template: template,
			name: "test1.nuodb.com",
		},"test1.nuodb.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_,_, err = reconcileNuodbSecret(cl, s, req, instance, tt.in, namespace)
			var secret = &corev1.Secret{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, secret)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test secret not found : (%v)", err)
			}

			if !reflect.DeepEqual(secret.ObjectMeta.Name,tt.out) {
				t.Errorf("Created secret map doesnt has same name (%v)", secret.Name)
			}
		})
	}
}

func Test_createNuodbPod(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  labels:
    app: myapp
spec:
  containers:
  - name: myapp-container
    image: busybox
    command: ['sh', '-c', 'echo Hello Kubernetes! && sleep 3600']`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Pod",NuoResource{
			template: template,
			name: "test-pod",
		},"busybox"},
		{"Test With Error New Pod",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbPod(cl, s, req, instance, tt.in)
			var pod = &corev1.Pod{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, pod)
			if err != nil {
				//Case where the given pod is not found by the reconcile funciton
				//and new pod cannot be created due to an error in the template
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test pod not found : (%v)", err)
			}

			if!reflect.DeepEqual(pod.Spec.Containers[0].Image,tt.out){
				t.Errorf("reconcilePod(%s) got %v, want %v", tt.in, pod, tt.out)
			}
		})
	}
}

func Test_reconcileNuodbPod(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  labels:
    app: myapp
spec:
  containers:
  - name: myapp-container
    image: busybox
    command: ['sh', '-c', 'echo Hello Kubernetes! && sleep 3600']`



	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Pod",NuoResource{
			template: template,
			name: "test-pod",
		},"busybox"},
		{"Test With Existing Pod",NuoResource{
			name: "nuodb-insights",
		} ,"nuodb/nuodb-ce:latest"},
		{"Test With Error New Pod",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_,_, err = reconcileNuodbPod(cl, s, req, instance, tt.in, namespace)
			var pod = &corev1.Pod{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, pod)
			if err != nil {
				//Case where the given pod is not found by the reconcile funciton
				//and new pod cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test pod not found : (%v)", err)
			}

			if!reflect.DeepEqual(pod.Spec.Containers[0].Image,tt.out){
				t.Errorf("reconcilePod(%s) got %v, want %v", tt.in, pod, tt.out)
			}
		})
	}
}

func Test_processTemplates(t *testing.T){

	instance, _ := getnuodbv2alpha1NuodbInstance(r, req)
	var nuoResources NuoResources

	var tests = []struct {
		name string
		in  string
		out string
	}{
		{"Test with Correct directory",utils.NuodbChartDir, "nuodb-ce-helm/templates/sts-sm.yaml"},
		{"Incorrect_directory","" ,""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nuoResources,_ = processNuodbTemplates(tt.in, instance.Spec)

			_, found := nuoResources.values[tt.out]
			if !found{
				if tt.name == "Incorrect_directory"{
					return
				}
				t.Fatal("Error in process template")
			}

		})
	}
}

func Test_getDaemonSet(t *testing.T){
	var secret = &appsv1.DaemonSet{}
	secret, err := utils.GetDaemonSet(cl, namespace,"")
	if err != nil{
		t.Fatalf("Get secret error : (%v)", err)
	}

	if secret.ObjectMeta.Name != "thp-disable"{
		t.Errorf("DaemonSet  name (%v) is not as expected ", secret.ObjectMeta.Name)
	}
}

func Test_createNuodbDaemonSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`apiVersion: apps/v1
kind: DaemonSet # it is a daemonset
metadata:
  name: test-daemonset # name of the daemon set
  labels:
    # any Pods with matching labels are included in this Daemon Set
    app: admin
    tier: monitor
spec:
  selector:
    # Pods will match with the following labels
    matchLabels:
      name: test-daemonset
  template:
    # Pod Template
    metadata:
      # Pod's labels
      labels:
        name: test-daemonset
    spec:
      containers:
        - name: daemon-container
          image: busybox:1.28`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new daemonset",NuoResource{
			template: template,
			name: "test-daemonset",
		},"test-daemonset"},
		{"Test With Error daemonset",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbDaemonSet(cl, s, req, instance, tt.in)
			var daemonSet = &appsv1.DaemonSet{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, daemonSet)
			if err != nil {
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test daemonSet not found : (%v)", err)
			}

			if !reflect.DeepEqual(daemonSet.ObjectMeta.Name,tt.out) {
				t.Errorf("Created daemonSet doesnt has same name (%v)", daemonSet.Name)
			}
		})
	}

}

func Test_reconcileNuodbDaemonSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	if err != nil {
		t.Fatalf("Error while getting instance : (%v)", err)
	}
	template :=`apiVersion: apps/v1
kind: DaemonSet # it is a daemonset
metadata:
  name: test-daemonset # name of the daemon set
  labels:
    # any Pods with matching labels are included in this Daemon Set
    app: admin
    tier: monitor
spec:
  selector:
    # Pods will match with the following labels
    matchLabels:
      name: test-daemonset
  template:
    # Pod Template
    metadata:
      # Pod's labels
      labels:
        name: test-daemonset
    spec:
      containers:
        - name: daemon-container
          image: busybox:1.28`

	var tests = []struct {
		name string
		in  NuoResource
		out string
	}{
		{"Test With new Secret",NuoResource{
			template: template,
			name: "test-daemonset",
		},"test-daemonset"},
		{"Existing-secret",NuoResource{
			template: template,
			name: "thp-disable",
		},"thp-disable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_,_, err = reconcileNuodbDaemonSet(cl, s, req, instance, tt.in, namespace)
			var daemonSet = &appsv1.DaemonSet{}
			err := cl.Get(context.TODO(), types.NamespacedName{Name: tt.in.name, Namespace: "nuodb"}, daemonSet)
			if err != nil {
				//Case where the given Deployment is not found by the get funciton
				//and new Deployment cannot be created
				sErr, ok := err.(*apierrors.StatusError)
				if ok && sErr.Status().Reason == "NotFound" {
					return
				}
				t.Fatalf("Test daemonSet not found : (%v)", err)
			}

			if !reflect.DeepEqual(daemonSet.ObjectMeta.Name,tt.out) {
				t.Errorf("Created daemonSet doesnt has same name (%v)", daemonSet.Name)
			}
		})
	}
}

func Test_createNuodbReplicationController(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
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
		in  NuoResource
		out string
	}{
		{"Test With new replicationcontroller",NuoResource{
			template: template,
			name: "test-replicationcontroller",
		},"test-replicationcontroller"},
		{"Test With Error replicationcontroller",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err = createNuodbReplicationController(cl, s, req, instance, tt.in)
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
