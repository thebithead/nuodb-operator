#!/bin/bash
PROJECT=nuodb
NODE=minikube
TESTDIR=$TRAVIS_BUILD_DIR
OPERATOR_NAMESPACE=nuodb

kubectl get nodes

kubectl label node ${NODE} nuodb.com/node-type=storage
kubectl label node ${NODE} nuodb.com/zone=nuodb --overwrite=true

cd ${TESTDIR}

export OPERATOR_NAMESPACE=nuodb

kubectl create namespace $OPERATOR_NAMESPACE

kubectl create secret docker-registry regcred --namespace=nuodb --docker-server=$DOCKER_SERVER --docker-username=$BOT_U --docker-password=$BOT_P --docker-email=""

#operator-sdk test local ./test/e2e --namespace $OPERATOR_NAMESPACE --verbose --kubeconfig $HOME/.kube/config --image $NUODB_OP_IMAGE

cd ${TESTDIR}/deploy

kubectl create -n $OPERATOR_NAMESPACE -f role.yaml
kubectl create -n $OPERATOR_NAMESPACE -f role_binding.yaml
kubectl create -n $OPERATOR_NAMESPACE -f service_account.yaml
kubectl patch serviceaccount nuodb-operator -p '{"imagePullSecrets": [{"name": "regcred"}]}' -n $OPERATOR_NAMESPACE
kubectl create -f crds/nuodb_v2alpha1_nuodb_crd.yaml
kubectl create -f crds/nuodb_v2alpha1_nuodbycsbwl_crd.yaml
kubectl create -f crds/nuodb_v2alpha1_nuodbinsightsserver_crd.yaml

sed -i "s|REPLACE_IMAGE|$NUODB_OP_IMAGE|" operator.yaml
kubectl create  -n $OPERATOR_NAMESPACE -f operator.yaml

# Check deployment rollout status every 10 seconds (max 10 minutes) until complete.
ATTEMPTS=0
ROLLOUT_STATUS_CMD="kubectl rollout status deployment/nuodb-operator -n nuodb"
until $ROLLOUT_STATUS_CMD || [ $ATTEMPTS -eq 60 ]; do
  $ROLLOUT_STATUS_CMD
  ATTEMPTS=$((attempts + 1))
  kubectl get pods -n nuodb
  sleep 10
done

kubectl create configmap nuodb-lic-configmap --from-literal=nuodb.lic="" -n $OPERATOR_NAMESPACE

echo "Create the Custom Resource to deploy NuoDB..."
kubectl create -n $OPERATOR_NAMESPACE -f ${TESTDIR}/test/files/ci_nuodb_test_cr.yaml

echo "status of all pods"
kubectl get pods -n nuodb

echo "wait till admin is ready"
ATTEMPTS=0
ROLLOUT_STATUS_CMD="kubectl rollout status sts/admin -n nuodb"
until $ROLLOUT_STATUS_CMD || [ $ATTEMPTS -eq 60 ]; do
  $ROLLOUT_STATUS_CMD
  ATTEMPTS=$((attempts + 1))
  kubectl get pods -n nuodb
  sleep 10
done

kubectl get pods -n nuodb

kubectl exec -it admin-0 /bin/bash nuocmd show domain -n nuodb
