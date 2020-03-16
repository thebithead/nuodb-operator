package util

import (
	"fmt"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/labels"

	"runtime/debug"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func arePodConditionsMet(pod *corev1.Pod, condition corev1.PodConditionType,
	status corev1.ConditionStatus) bool {
	for _, cnd := range pod.Status.Conditions {
		if cnd.Type == condition && cnd.Status == status {
			fmt.Printf("Pod (%s) is %s\n", pod.Name, condition)
			return true
		}
	}

	return false
}

func FindAllPodsInSchema(t *testing.T, f *framework.Framework, namespace string) []*corev1.Pod {
	opts :=metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{}).String(),
	}

	podList, err := f.KubeClient.CoreV1().Pods(namespace).List(opts)
	assert.NilError(t, err)

	var pods []*corev1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase == corev1.PodRunning {
			pods = append(pods, pod)
		}
	}
	return  pods
}


func await(t *testing.T, lmbd func() bool, timeout time.Duration)  {
	for timeExpired := time.After(timeout); ; {
		select {
		case <-timeExpired:
			t.Log(string(debug.Stack()))
			t.Fatal("function call timed out")
		default:
			if lmbd() {
				return
			}

			time.Sleep(1 * time.Second)
		}
	}
}

func AwaitNrReplicasScheduled(t *testing.T, f *framework.Framework, namespace string, expectedName string, nrReplicas int) {
	await(t, func() bool {
		var cnt int
		for _, pod := range FindAllPodsInSchema(t, f, namespace) {
			if strings.Contains(pod.Name, expectedName) {
				if arePodConditionsMet(pod, corev1.PodScheduled, corev1.ConditionTrue) {
					cnt++
				}
			}
		}

		t.Logf("%d pods SCHEDULED for name '%s'\n", cnt, expectedName)

		return cnt == nrReplicas
	}, 30*time.Second)
}

func awaitPodStatus(t *testing.T, f *framework.Framework, namespace string, podName string, condition corev1.PodConditionType,
	status corev1.ConditionStatus, timeout time.Duration) {
	await(t, func() bool {
		pod, err :=  f.KubeClient.CoreV1().Pods(namespace).Get(podName,metav1.GetOptions{} ) //k8s.GetPod(t, options, podName)
		assert.NilError(t, err)
		t.Log("Awaiting pod status", podName, condition, status)
		return arePodConditionsMet(pod, condition, status)
	}, timeout)
}

func AwaitAdminPodUp(t *testing.T, f *framework.Framework, namespace string, adminPodName string, timeout time.Duration) {
	awaitPodStatus(t, f, namespace, adminPodName, corev1.PodReady, corev1.ConditionTrue, timeout)
}

func AwaitPodRestartCountGreaterThan(t *testing.T, f *framework.Framework, namespace string, podName string, expectedRestartCount int32) {
	await(t, func() bool {
		pod, err :=  f.KubeClient.CoreV1().Pods(namespace).Get(podName,metav1.GetOptions{} ) //k8s.GetPod(t, options, podName)
		assert.NilError(t, err)

		var restartCount int32
		for _, status := range pod.Status.ContainerStatuses {
			restartCount += status.RestartCount
		}

		return restartCount > expectedRestartCount
	}, 30*time.Second)
}

func VerifyAdminState(t *testing.T, f *framework.Framework, namespace string, podName string, containerName string) {
	command := []string{"nuocmd", "show", "domain"}
	testOutput, err := ExecCommand(f, namespace ,podName , containerName,command)
	t.Log("Output show domain",testOutput)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(testOutput, "ACTIVE"))
}

func KillAdminPod(t *testing.T, f *framework.Framework, namespace string, podName string) {
	podInterface := f.KubeClient.CoreV1().Pods(namespace)
	err := podInterface.Delete(podName, &metav1.DeleteOptions{})
	assert.NilError(t, err)

	time.Sleep(10 * time.Second) // wait for the pod to restart, TODO this could be improved to an await
}

func KillAdminProcess(t *testing.T, f *framework.Framework, namespace string, podName string) {
	fmt.Printf("Killing pid 1 in pod %s\n", podName)
	command := []string{"kill", "1"}
	_, err := ExecCommand(f, namespace ,podName ,"admin", command)
	assert.NilError(t, err)

	AwaitPodRestartCountGreaterThan(t, f, namespace, podName, 0)
}

func PingService(t *testing.T, f *framework.Framework, namespace string, serviceName string, podName string) {
	command := []string{"ping", serviceName + ".nuodb.svc.cluster.local", "-c", "1"}
	output, err := ExecCommand(f, namespace ,podName , "admin" ,command)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(output, "1 received"))
}
