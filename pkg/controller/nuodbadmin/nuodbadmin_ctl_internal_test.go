package nuodbadmin

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
var r = &ReconcileNuodbAdmin{client: cl, scheme: s}
var req = reconcile.Request{}

func init() {
	var (
		name                    = "nuodb-operator"
		namespace               = "nuodb"
		storageMode             = "ephemeral"
		adminCount        int32 = 1
		adminStorageSize        = "5G"
		adminStorageClass       = "local-disk"
		apiServer               = "https://domain:8888"
		container               = "nuodb/nuodb-ce:latest"
	)

	logf.SetLogger(logf.ZapLogger(true))
	nuodbAdmin := &nuodbv2alpha1.NuodbAdmin{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: nuodbv2alpha1.NuodbAdminSpec{
			StorageMode:       storageMode,
			InsightsEnabled:   true,
			AdminCount:        adminCount,
			AdminStorageSize:  adminStorageSize,
			AdminStorageClass: adminStorageClass,
			ApiServer:         apiServer,
			Container:         container,
		},
		Status: nuodbv2alpha1.NuodbAdminStatus{
			ControllerVersion: utils.NuodbOperatorVersion,
			Phase:             utils.NuodbPendingPhase,
			AdminReadyCount:   0,
			AdminHealth:       utils.NuodbUnknownHealth,
			DomainHealth:      utils.NuodbUnknownHealth,
		},
	}

	nl := [1] nuodbv2alpha1.NuodbAdmin {*nuodbAdmin}
	nuodbAdminList := &nuodbv2alpha1.NuodbAdminList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "nuodbs.nuodbAdmin.com/v2alpha1",
			Kind:       "NuodbAdminList",
		},
		ListMeta: metav1.ListMeta{},
		Items:    nl[1:1],
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{
		nuodbAdmin,
		nuodbAdminList,
	}

	// Register operator types with the runtime scheme.
	s = scheme.Scheme

	s.AddKnownTypes(nuodbv2alpha1.SchemeGroupVersion, nuodbAdmin)
	s.AddKnownTypes(nuodbv2alpha1.SchemeGroupVersion, nuodbAdminList)
	// Create a fake client to mock API calls.
	cl = fake.NewFakeClient(objs...)

	r = &ReconcileNuodbAdmin{client: cl, scheme: s}

	req = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	_, err := reconcileNuodbAdminInternal(r, req)
	if err != nil {
		fmt.Printf(" Reconcile Failed with error  %v\n", err)
	}
}

func Test_getService(t *testing.T){
	var service = &corev1.Service{}
	service,err := utils.GetService(cl, namespace, "admin")
	if err != nil {
		t.Fatalf("Get Service error : (%v)", err)
	}
	if !reflect.DeepEqual(service.Spec.Selector, map[string]string{"app": "admin"}){
		t.Fatalf("Expect %+v", service)
	}
}

func Test_createNuodbService(t *testing.T) {
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
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
			_, err = createNuodbAdminService(cl, s, req, instance, tt.in)
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
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
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
		{"Test With Existing Service",NuoResource{
			name: "admin",
		} ,"admin"},
		{"Test With Error Service",NuoResource{
			template: "",
			name: "xyz",
		} ,"NotFound"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_,_, err = reconcileNuodbAdminService(cl, s, req, instance, tt.in, namespace)
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


func Test_getStatefulSet(t *testing.T){
	var sts = &appsv1.StatefulSet{}
	sts, err := utils.GetStatefulSetV1(cl, namespace,"admin")
	if err != nil{
		t.Fatalf("Get StatefulSet error : (%v)", err)
	}

	if *sts.Spec.Replicas != int32(1){
		t.Errorf("Stateful AdminCount size (%d) is not the expected size (%d)", sts.Spec.Replicas, int32(1))
	}
}

func Test_createNuodbStatefulSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
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
			_, err = createNuodbAdminStatefulSet(cl, s, req, instance, tt.in)
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

	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
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
			name: "admin",
		},1},
		{"Update-Count-admin",NuoResource{
			name: "admin",
		} ,3},
		{"Update-Count-sm",NuoResource{
			name: "sm",
		} ,3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name=="Update-Count-admin"{
				instance.Spec.AdminCount=3
			}
			_,_, err = reconcileNuodbAdminStatefulSet(cl, s, req, instance, tt.in, namespace)
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

func Test_getConfigMap(t *testing.T){
	var configMap = &corev1.ConfigMap{}
	configMap, err := utils.GetConfigMap(cl, namespace,"insights-configmap")
	if err != nil{
		t.Fatalf("Get Configmap error : (%v)", err)
	}

	if configMap.ObjectMeta.Name != "insights-configmap"{
		t.Errorf("ConfigMap  name (%v) is not as expected ", configMap.ObjectMeta.Name)
	}
}

func Test_createNuodbConfigMap(t *testing.T){

	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
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
			_, err = createNuodbAdminConfigMap(cl, s, req, instance, tt.in)
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
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
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
			_,_, err = reconcileNuodbAdminConfigMap(cl, s, req, instance, tt.in, namespace)
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

func Test_getPod(t *testing.T){
	var pod = &corev1.Pod{}
	pod, err := utils.GetPod(cl, namespace,"nuodb-insights")
	if err != nil{
		t.Fatalf("Get Pod error : (%v)", err)
	}

	if pod.ObjectMeta.Name != "nuodb-insights"{
		t.Errorf("Pod  name (%v) is not as expected ", pod.ObjectMeta.Name)
	}
}

func Test_createNuodbPod(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
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
			_, err = createNuodbAdminPod(cl, s, req, instance, tt.in)
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
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
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
			_,_, err = reconcileNuodbAdminPod(cl, s, req, instance, tt.in, namespace)
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

	instance, _ := getnuodbv2alpha1NuodbAdminInstance(r, req)
	var nuoResources NuoResources

	var tests = []struct {
		name string
		in  string
		out string
	}{
		{"Test with Correct directory",utils.NuodbAdminChartDir, "nuodbadmin-helm/templates/sts-admin.yaml"},
		{"Incorrect_directory","" ,""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nuoResources,_ = processNuodbAdminTemplates(tt.in, instance.Spec)

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

