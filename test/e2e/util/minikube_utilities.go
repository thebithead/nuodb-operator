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
	if err != nil {
		t.Log("Error in Finding all pods")
		return nil
	}

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
	var cnt int
	time.Sleep(60 * time.Second)
	for _, pod := range FindAllPodsInSchema(t, f, namespace) {
		if strings.Contains(pod.Name, expectedName) {
			if arePodConditionsMet(pod, corev1.PodScheduled, corev1.ConditionTrue) {
				cnt++
			}
		}
	}
	assert.Equal(t, cnt, nrReplicas)
	t.Log(fmt.Printf("%d pods SCHEDULED for name '%s'\n", cnt, expectedName))
}

func awaitPodStatus(t *testing.T, f *framework.Framework, namespace string, podName string, condition corev1.PodConditionType,
	status corev1.ConditionStatus, timeout time.Duration) {
	await(t, func() bool {
		pod, _ :=  f.KubeClient.CoreV1().Pods(namespace).Get(podName,metav1.GetOptions{} ) //k8s.GetPod(t, options, podName)
		t.Log("Awaiting pod status", podName, condition, status)
		return arePodConditionsMet(pod, condition, status)
	}, timeout)
}

func AwaitAdminPodUp(t *testing.T, f *framework.Framework, namespace string, adminPodName string, timeout time.Duration) {
	awaitPodStatus(t, f, namespace, adminPodName, corev1.PodReady, corev1.ConditionTrue, timeout)
}

func AwaitBalancerTerminated(t *testing.T,f *framework.Framework, namespace string, expectedName string) {
	await(t, func() bool {
		for _, pod := range FindAllPodsInSchema(t, f, namespace) {
			if strings.Contains(pod.Name, expectedName) {
				if pod.Status.Phase == "Succeeded" {
					fmt.Printf("Pod (%s) TERMINATED\n", expectedName)
					return true
				}
			}
		}
		return false
	}, 30 * time.Second)
}
//
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
}

func KillAdminProcess(t *testing.T, f *framework.Framework, namespace string, podName string) {
	command := []string{"ps", "-ef"}
	output, err := ExecCommand(f, namespace ,podName , "admin",command)
	//output, err := k8s.RunKubectlAndGetOutputE(t, options, "exec", podName, "--", "ps")
	assert.NilError(t, err)
	parts:= strings.Split(output, "\n")

	var pid string
	for _, part := range parts {
		if strings.Contains(part, "java") {
			pid = strings.Fields(part)[1]
		}
	}
	assert.Assert(t, pid != "", "pid not found in :%s\n", output)

	fmt.Printf("Killing pid %s in pod %s\n", pid, podName)

	command = []string{"kill", pid}
	output, err = ExecCommand(f, namespace ,podName , "admin",command)
	assert.NilError(t, err)
}

func PingService(t *testing.T, f *framework.Framework, namespace string, serviceName string, podName string) {
	command := []string{"ping", serviceName + ".nuodb.svc.cluster.local", "-c", "1"}
	output, err := ExecCommand(f, namespace ,podName , "admin" ,command)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(output, "1 received"))
}
