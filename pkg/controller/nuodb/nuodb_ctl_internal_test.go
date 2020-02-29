package nuodb

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
		_, err = createNuodbService(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-service",
		})
		assert.NilError(t, err)

		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-service", Namespace: "nuodb"}, service)
		assert.NilError(t, err)
		assert.Equal(t, service.Spec.Selector["group"], "nuodb")
	})

	t.Run("Test With Error Service", func(t *testing.T) {
		_, err = createNuodbService(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})

}

func Test_reconcileNuodbService(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
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
		_,_, err = reconcileNuodbService(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-service",
		}, namespace)
		assert.NilError(t, err)

		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-service", Namespace: "nuodb"}, service)
		assert.NilError(t, err)
		assert.Equal(t, service.Spec.Selector["group"], "nuodb")
	})

	t.Run("Test With Error Service", func(t *testing.T) {
		_,_, err = reconcileNuodbService(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		}, namespace)
		assert.Assert(t, err != nil)
	})
}

func Test_getDeployment(t *testing.T)  {
	var deployment *appsv1.Deployment = nil
	deployment, err := utils.GetDeployment(cl, namespace, "nuodb-operator-te")
	assert.NilError(t, err)
	assert.Equal(t, *deployment.Spec.Replicas, int32(1))
}

func Test_createNuodbDeployment(t *testing.T) {
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	assert.NilError(t, err)
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


	t.Run("Test With new Deployment", func(t *testing.T) {
		_, err = createNuodbDeployment(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-deployment",
		})
		assert.NilError(t, err)

		var deployment = &appsv1.Deployment{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-deployment", Namespace: "nuodb"}, deployment)
		assert.NilError(t, err)
		assert.Equal(t, *deployment.Spec.Replicas, int32(3))
	})

	t.Run("Test With error new Deployment", func(t *testing.T) {
		_, err = createNuodbDeployment(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})
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

	t.Run("Test With new Deployment", func(t *testing.T) {
		_,_, err = reconcileNuodbDeployment(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-deployment",
		}, namespace)
		assert.NilError(t, err)

		var deployment = &appsv1.Deployment{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-deployment", Namespace: "nuodb"}, deployment)
		assert.NilError(t, err)
		assert.Equal(t, *deployment.Spec.Replicas, int32(3))
	})

	t.Run("Test With existing Deployment", func(t *testing.T) {
		_,_, err = reconcileNuodbDeployment(cl, s, req, instance, NuoResource{
			template: template,
			name: "nuodb-operator-te",
		}, namespace)
		assert.NilError(t, err)

		var deployment = &appsv1.Deployment{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "nuodb-operator-te", Namespace: "nuodb"}, deployment)
		assert.NilError(t, err)
		assert.Equal(t, *deployment.Spec.Replicas, int32(1))
	})

	t.Run("Update-count", func(t *testing.T) {
		t.Skip("Broken")
		instance.Spec.TeCount=3
		defer func() {
			instance.Spec.TeCount=1
		}()

		_,_, err = reconcileNuodbDeployment(cl, s, req, instance, NuoResource{
			template: template,
			name: "nuodb-operator-te",
		}, namespace)
		assert.NilError(t, err)

		var deployment = &appsv1.Deployment{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "nuodb-operator-te", Namespace: "nuodb"}, deployment)
		assert.NilError(t, err)
		assert.Equal(t, *deployment.Spec.Replicas, int32(3))
	})
}

func Test_createNuodbStatefulSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
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
		_, err = createNuodbStatefulSet(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-statefulset",
		})
		var statefulSet = &appsv1.StatefulSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-statefulset", Namespace: "nuodb"}, statefulSet)
		assert.NilError(t, err)
		assert.Equal(t, *statefulSet.Spec.Replicas, int32(2))
	})

	t.Run("Test With Error StatefulSet", func(t *testing.T) {
		_, err = createNuodbStatefulSet(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})
}

func Test_reconcileNuodbStatefulSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
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
		_,_, err = reconcileNuodbStatefulSet(cl, s, req, instance, NuoResource{
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
		_,_, err = reconcileNuodbStatefulSet(cl, s, req, instance, NuoResource{
			template: template,
			name: "nuodb-operator-sm",
		}, namespace)
		assert.NilError(t, err)
		var statefulSet = &appsv1.StatefulSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "nuodb-operator-sm", Namespace: "nuodb"}, statefulSet)
		assert.NilError(t, err)
		assert.Equal(t, *statefulSet.Spec.Replicas, int32(1))
	})

	t.Run("Update-Count-sm", func(t *testing.T) {
		t.Skip("Broken DB-30302")
		instance.Spec.SmCount=3
		defer func() {
			instance.Spec.SmCount=1
		}()

		_,_, err = reconcileNuodbStatefulSet(cl, s, req, instance, NuoResource{
			name: "nuodb-operator-sm",
		}, namespace)
		assert.NilError(t, err)
		var statefulSet = &appsv1.StatefulSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "nuodb-operator-sm", Namespace: "nuodb"}, statefulSet)
		assert.NilError(t, err)
		assert.Equal(t, *statefulSet.Spec.Replicas, int32(3))
	})

}

func Test_createNuodbConfigMap(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	assert.NilError(t, err)

	template :=`kind: ConfigMap 
apiVersion: v1 
metadata:
  name: test-configmap 
data:
  database: nuodb`

	t.Run("Test With new Configmap", func(t *testing.T) {
		_, err = createNuodbConfigMap(cl, s, req, instance, NuoResource{
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
		_, err = createNuodbConfigMap(cl, s, req, instance, NuoResource{
			name: "test-configmap",
		})
		assert.Assert(t, err != nil)
	})

}

func Test_reconcileNuodbConfigMap(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	assert.NilError(t, err)

	template :=`kind: ConfigMap 
apiVersion: v1 
metadata:
  name: test-configmap 
data:
  database: nuodb`

	t.Run("Test With new Configmap", func(t *testing.T) {
		_,_, err = reconcileNuodbConfigMap(cl, s, req, instance, NuoResource{
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
		t.Skip(t, "broken")
		_,_, err = reconcileNuodbConfigMap(cl, s, req, instance, NuoResource{
			template: template,
			name: "insights-configmap",
		}, namespace)
		assert.NilError(t, err)
		var configMap = &corev1.ConfigMap{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "insights-configmap", Namespace: "nuodb"}, configMap)
		assert.NilError(t, err)
	})
}

func Test_getSecret(t *testing.T){
	var secret = &corev1.Secret{}
	secret, err := utils.GetSecret(cl, namespace,"")
	assert.NilError(t, err)

	assert.Equal(t, secret.ObjectMeta.Name, "test1.nuodb.com")
}

func Test_createNuodbSecret(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	assert.NilError(t, err)

	template :=`apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: Opaque
data:
  test: test`

	t.Run("Test With new Secret", func(t *testing.T) {
		_, err = createNuodbSecret(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-secret",
		})
		assert.NilError(t, err)
		var secret = &corev1.Secret{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-secret", Namespace: "nuodb"}, secret)
		assert.NilError(t, err)
	})

	t.Run("Test With Error Secret", func(t *testing.T) {
		_, err = createNuodbSecret(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})

}

func Test_reconcileNuodbSecret(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	assert.NilError(t, err)

	template :=`apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: Opaque
data:
  test: test`

	t.Run("Test With new Secret", func(t *testing.T) {
		_,_, err = reconcileNuodbSecret(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-secret",
		}, namespace)
		assert.NilError(t, err)
		var secret = &corev1.Secret{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-secret", Namespace: "nuodb"}, secret)
		assert.NilError(t, err)
	})

	t.Run("Test With existing secret", func(t *testing.T) {
		_,_, err = reconcileNuodbSecret(cl, s, req, instance, NuoResource{
			template: template,
			name: "test1.nuodb.com",
		}, namespace)
		assert.NilError(t, err)
		var secret = &corev1.Secret{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test1.nuodb.com", Namespace: "nuodb"}, secret)
		assert.NilError(t, err)
	})
}

func Test_createNuodbPod(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
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
		_, err = createNuodbPod(cl, s, req, instance, NuoResource{
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
		_, err = createNuodbPod(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})
}

func Test_reconcileNuodbPod(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
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
		_,_, err = reconcileNuodbPod(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-pod",
		}, namespace)
		assert.NilError(t, err)

		var pod = &corev1.Pod{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-pod", Namespace: "nuodb"}, pod)
		assert.NilError(t, err)
		assert.Equal(t, pod.Spec.Containers[0].Image, "busybox")
	})

	t.Run("Test With error new Pod", func(t *testing.T) {
		_, _, err = reconcileNuodbPod(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		}, namespace)
		assert.Assert(t, err != nil)
	})
}

func Test_processTemplates(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	assert.NilError(t, err)

	t.Run("Test with Correct directory", func(t *testing.T) {
		nuoResources, err := processNuodbTemplates(utils.NuodbAdminChartDir, instance.Spec)
		assert.NilError(t, err)

		_, found := nuoResources.values["nuodbadmin-helm/templates/sts-admin.yaml"]
		assert.Check(t, found, "nuodbadmin-helm/templates/sts-admin.yaml could not be found")
	})

	t.Run("Test with Correct directory", func(t *testing.T) {
		_, err := processNuodbTemplates("", instance.Spec)
		assert.Assert(t, err != nil)
	})
}

func Test_getDaemonSet(t *testing.T){
	var secret = &appsv1.DaemonSet{}
	secret, err := utils.GetDaemonSet(cl, namespace,"")
	assert.NilError(t, err)
	assert.Equal(t, secret.ObjectMeta.Name,  "thp-disable")
}

func Test_createNuodbDaemonSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	assert.NilError(t, err)

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

	t.Run("Test With new DaemonSet", func(t *testing.T) {
		_,_, err = reconcileNuodbDaemonSet(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-daemonset",
		}, namespace)
		assert.NilError(t, err)

		var daemonSet = &appsv1.DaemonSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-daemonset", Namespace: "nuodb"}, daemonSet)
		assert.NilError(t, err)
		assert.Equal(t, daemonSet.ObjectMeta.Name, "test-daemonset")
	})

	t.Run("Test With error new DaemonSet", func(t *testing.T) {
		_, _, err = reconcileNuodbDaemonSet(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		}, namespace)
		assert.Assert(t, err != nil)
	})

}

func Test_reconcileNuodbDaemonSet(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
	assert.NilError(t, err)

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

	t.Run("Test With new DaemonSet", func(t *testing.T) {
		_,_, err = reconcileNuodbDaemonSet(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-daemonset",
		}, namespace)
		assert.NilError(t, err)

		var daemonSet = &appsv1.DaemonSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-daemonset", Namespace: "nuodb"}, daemonSet)
		assert.NilError(t, err)
		assert.Equal(t, daemonSet.ObjectMeta.Name, "test-daemonset")
	})

	t.Run("Test With existing DaemonSet", func(t *testing.T) {
		_,_, err = reconcileNuodbDaemonSet(cl, s, req, instance, NuoResource{
			template: template,
			name: "thp-disable",
		}, namespace)
		assert.NilError(t, err)

		var daemonSet = &appsv1.DaemonSet{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "thp-disable", Namespace: "nuodb"}, daemonSet)
		assert.NilError(t, err)
		assert.Equal(t, daemonSet.ObjectMeta.Name, "thp-disable")
	})

}

func Test_createNuodbReplicationController(t *testing.T){
	instance, err := getnuodbv2alpha1NuodbInstance(r, req)
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

	t.Run("Test With new ReplicationController", func(t *testing.T) {
		_, err = createNuodbReplicationController(cl, s, req, instance, NuoResource{
			template: template,
			name: "test-replicationcontroller",
		})
		assert.NilError(t, err)

		var replicationController = &corev1.ReplicationController{}
		err := cl.Get(context.TODO(), types.NamespacedName{Name: "test-replicationcontroller", Namespace: "nuodb"}, replicationController)
		assert.NilError(t, err)
		assert.Equal(t, replicationController.ObjectMeta.Name, "test-replicationcontroller")
	})

	t.Run("Test With error new ReplicationController", func(t *testing.T) {
		_, err = createNuodbReplicationController(cl, s, req, instance, NuoResource{
			template: "",
			name: "xyz",
		})
		assert.Assert(t, err != nil)
	})
}
