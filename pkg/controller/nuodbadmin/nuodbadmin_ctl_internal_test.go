package nuodbadmin

import (
	"context"
	"fmt"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	nuodbv2alpha1 "nuodb/nuodb-operator/pkg/apis/nuodb/v2alpha1"
	"nuodb/nuodb-operator/pkg/utils"
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
			Phase:             nuodbv2alpha1.NuodbPendingPhase,
			AdminReadyCount:   0,
			AdminHealth:       nuodbv2alpha1.NuodbUnknownHealth,
			DomainHealth:      nuodbv2alpha1.NuodbUnknownHealth,
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
	service, err := utils.GetService(cl, namespace, "admin")
	assert.NilError(t, err)
	assert.Equal(t, service.Spec.Selector["app"], "admin")
}

func Test_createNuodbService(t *testing.T) {
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

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

	t.Run("Test With new Service", func(t *testing.T) {
		_, err = createNuodbAdminService(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-service",
		})
		assert.NilError(t, err)

		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-service", Namespace: "nuodb"}, service)
		assert.NilError(t, err)
		assert.Equal(t, service.Spec.Selector["group"], "nuodb")
	})

	t.Run("Test With Error Service", func(t *testing.T) {
		_, err = createNuodbAdminService(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})

}

func Test_reconcileNuodbService(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

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

	t.Run("Test With new Service", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminService(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-service",
		}, namespace)
		assert.NilError(t, err)

		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-service", Namespace: "nuodb"}, service)
		assert.NilError(t, err)
		assert.Equal(t, service.Spec.Selector["group"], "nuodb")
	})

	t.Run("Test With existing Service", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminService(cl, s, req, instance, NuoResource{
			name: "admin",
		}, namespace)
		assert.NilError(t, err)

		err := cl.Get(context.TODO(), types.NamespacedName{Name: "admin", Namespace: "nuodb"}, service)
		assert.NilError(t, err)
		assert.Equal(t, service.Spec.Selector["app"], "admin")
	})

	t.Run("Test With Error Service", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminService(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		}, namespace)
		assert.Assert(t, err != nil)
	})
}


func Test_getStatefulSet(t *testing.T){
	var sts = &appsv1.StatefulSet{}
	sts, err := utils.GetStatefulSetV1(cl, namespace,"admin")
	assert.NilError(t, err)
	assert.Equal(t, *sts.Spec.Replicas, int32(1))
}

func Test_createNuodbStatefulSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

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

	t.Run("Test With new StatefulSet", func(t *testing.T) {
		_, err = createNuodbAdminStatefulSet(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-statefulset",
		})
		var statefulSet = &appsv1.StatefulSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-statefulset", Namespace: "nuodb"}, statefulSet)
		assert.NilError(t, err)
		assert.Equal(t, *statefulSet.Spec.Replicas, int32(2))
	})

	t.Run("Test With Error StatefulSet", func(t *testing.T) {
		_, err = createNuodbAdminStatefulSet(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})
}

func Test_reconcileNuodbStatefulSet(t *testing.T){

	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

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

	t.Run("Test With new Statefulset", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminStatefulSet(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-statefulset",
		}, namespace)
		assert.NilError(t, err)
		var statefulSet = &appsv1.StatefulSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-statefulset", Namespace: "nuodb"}, statefulSet)
		assert.NilError(t, err)
		assert.Equal(t, *statefulSet.Spec.Replicas, int32(2))
	})

	t.Run("Default-Statefulset", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminStatefulSet(cl, s, req, instance, NuoResource{
			template: template,
			name: "admin",
		}, namespace)
		assert.NilError(t, err)
		var statefulSet = &appsv1.StatefulSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "admin", Namespace: "nuodb"}, statefulSet)
		assert.NilError(t, err)
		assert.Equal(t, *statefulSet.Spec.Replicas, int32(1))
	})

	t.Run("Update-Count-admin", func(t *testing.T) {
		instance.Spec.AdminCount=3
		defer func() {
			instance.Spec.AdminCount=1
		}()

		_,_, err = reconcileNuodbAdminStatefulSet(cl, s, req, instance, NuoResource{
			name: "admin",
		}, namespace)
		assert.NilError(t, err)
		var statefulSet = &appsv1.StatefulSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "admin", Namespace: "nuodb"}, statefulSet)
		assert.NilError(t, err)
		assert.Equal(t, *statefulSet.Spec.Replicas, int32(3))
	})

	t.Run("Update-Count-sm", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminStatefulSet(cl, s, req, instance, NuoResource{
			name: "sm",
		}, namespace)
		assert.Assert(t, err != nil)
	})

}

func Test_getConfigMap(t *testing.T){
	var configMap = &corev1.ConfigMap{}
	configMap, err := utils.GetConfigMap(cl, namespace,"insights-configmap")
	assert.NilError(t, err)
	assert.Equal(t, configMap.ObjectMeta.Name, "insights-configmap")
}

func Test_createNuodbConfigMap(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

	template :=`kind: ConfigMap 
apiVersion: v1 
metadata:
  name: test-configmap 
data:
  database: nuodb`

	t.Run("Test With new Configmap", func(t *testing.T) {
		_, err = createNuodbAdminConfigMap(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-configmap",
		})
		assert.NilError(t, err)
		var configMap = &corev1.ConfigMap{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-configmap", Namespace: "nuodb"}, configMap)
		assert.NilError(t, err)
		assert.Equal(t, configMap.Data["database"], "nuodb")
	})

	t.Run("Test With Error Configmap", func(t *testing.T) {
		_, err = createNuodbAdminConfigMap(cl, s, req, instance, NuoResource{
			name: "test-configmap",
		})
		assert.Assert(t, err != nil)
	})

}

func Test_reconcileNuodbConfigMap(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

	template :=`kind: ConfigMap 
apiVersion: v1 
metadata:
  name: test-configmap 
data:
  database: nuodb`


	t.Run("Test With new Configmap", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminConfigMap(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-configmap",
		}, namespace)
		assert.NilError(t, err)
		var configMap = &corev1.ConfigMap{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-configmap", Namespace: "nuodb"}, configMap)
		assert.NilError(t, err)
		assert.Equal(t, configMap.Data["database"], "nuodb")
	})

	t.Run("Test With existing Configmap", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminConfigMap(cl, s, req, instance, NuoResource{
			template: template,
			name: "insights-configmap",
		}, namespace)
		assert.NilError(t, err)
		var configMap = &corev1.ConfigMap{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "insights-configmap", Namespace: "nuodb"}, configMap)
		assert.NilError(t, err)
	})
}

func Test_getPod(t *testing.T){
	var pod = &corev1.Pod{}
	pod, err := utils.GetPod(cl, namespace,"nuodb-insights")
	assert.NilError(t, err)
	assert.Equal(t, pod.ObjectMeta.Name, "nuodb-insights")
}

func Test_createNuodbPod(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

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

	t.Run("Test With new Pod", func(t *testing.T) {
		_, err = createNuodbAdminPod(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-pod",
		})
		assert.NilError(t, err)

		var pod = &corev1.Pod{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-pod", Namespace: "nuodb"}, pod)
		assert.NilError(t, err)
		assert.Equal(t, pod.Spec.Containers[0].Image, "busybox")
	})

	t.Run("Test With error new Pod", func(t *testing.T) {
		_, err = createNuodbAdminPod(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})
}

func Test_reconcileNuodbPod(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

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


	t.Run("Test With new Pod", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminPod(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-pod",
		}, namespace)
		assert.NilError(t, err)

		var pod = &corev1.Pod{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-pod", Namespace: "nuodb"}, pod)
		assert.NilError(t, err)
		assert.Equal(t, pod.Spec.Containers[0].Image, "busybox")
	})

	t.Run("Test With existing Pod", func(t *testing.T) {
		_,_, err = reconcileNuodbAdminPod(cl, s, req, instance, NuoResource{
			name: "nuodb-insights",
		}, namespace)
		assert.NilError(t, err)

		var pod = &corev1.Pod{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "nuodb-insights", Namespace: "nuodb"}, pod)
		assert.NilError(t, err)
		assert.Equal(t, pod.Spec.Containers[0].Image, "nuodb/nuodb-ce:latest")
	})

	t.Run("Test With error new Pod", func(t *testing.T) {
		_, _, err = reconcileNuodbAdminPod(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		}, namespace)
		assert.Assert(t, err != nil)
	})
}

func Test_processTemplates(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbAdminInstance(r, req)
	assert.NilError(t, err)

	t.Run("Test with Correct directory", func(t *testing.T) {
		nuoResources, err := processNuodbAdminTemplates(utils.NuodbAdminChartDir, instance.Spec)
		assert.NilError(t, err)

		_, found := nuoResources.values["nuodbadmin-helm/templates/sts-admin.yaml"]
		assert.Check(t, found, "nuodbadmin-helm/templates/sts-admin.yaml could not be found")
	})

	t.Run("Test with Correct directory", func(t *testing.T) {
		_, err := processNuodbAdminTemplates("", instance.Spec)
		assert.Assert(t, err != nil)
	})
}

