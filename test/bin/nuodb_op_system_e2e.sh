#!/bin/bash
#
# NuoDB Operator System End to End Test.
#
# To run this test, simply run it with no parameters.  The test expects
# a valid KUBECONFIG.
#
# To cleanup after a test run, run it with "clean" parameter.  Example:
#
#    nuodb_op_system_e2e.sh clean
#

if [ "$1" == "clean" ]; then
  echo "Cleaning up from previous test"
  kubectl delete pod/insights-client
  kubectl delete nuodbycsbwls/nuodbycsbwl-test1
  kubectl delete nuodbinsightsservers/insightsserver
  kubectl delete nuodbs/nuodb-test1
  kubectl delete nuodbs/nuodb-test2
  kubectl delete deployment/nuodb-operator
  kubectl delete crds/nuodbinsightsservers.nuodb.com
  kubectl delete crds/nuodbs.nuodb.com
  kubectl delete crds/nuodbycsbwls.nuodb.com
  kubectl delete crds/nuodbadmins.nuodb.com
  kubectl delete crds/grafanadashboards.integreatly.org
  kubectl delete crds/grafanadatasources.integreatly.org
  kubectl delete crds/grafanas.integreatly.org
  kubectl delete crds/apmservers.apm.k8s.elastic.co
  kubectl delete crds/elasticsearches.elasticsearch.k8s.elastic.co
  kubectl delete crds/kibanas.kibana.k8s.elastic.co
  kubectl delete sa/nuodb-operator
  kubectl delete roles/grafana-operator
  kubectl delete roles/nuodb-operator
  kubectl delete rolebinding/grafana-operator
  kubectl delete rolebinding/nuodb-operator
  kubectl delete clusterrolebinding/nuodb-op-admin
  kubectl delete secret/regcred
  kubectl config set-context --current --namespace=default
  kubectl delete ns/nuodb
  rm -fr nuodb-operator
  echo "Clean completed."
  exit 0  
fi

echo ""
echo "Checking KUBECONFIG environment variable..."
if [ -z "$KUBECONFIG" ]
then
  echo "KUBECONFIG environment variable is empty"
  echo "$0: FAIL"
  exit 1
fi

echo ""
echo "Checking NUOOPGITBRANCH environment variable..."
if [ -z "$NUOOPGITBRANCH" ]
then
  export NUOOPGITBRANCH=master
fi
echo "Using nuodb-operator branch: $NUOOPGITBRANCH"

echo ""
echo "Checking NUOOPIMAGE environment variable..."
if [ -z "$NUOOPIMAGE" ]
then
  export NUOOPIMAGE=nuodb-operator:latest
fi
echo "Using nuodb-operator image name: $NUOOPIMAGE"

echo ""
echo "Version info..."
kubectl version
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Cluster info..."
kubectl cluster-info
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Checking for OpenShift..."
oc status
retval=$?
if [ $retval -eq 0 ]; then
  echo "Cluster is OpenShift"
  openshift=1
else
  echo "Cluster is NOT OpenShift"
  openshift=0
fi

echo ""
echo "Creating namespace: nuodb ..."
kubectl create namespace nuodb
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi
kubectl config set-context --current --namespace=nuodb
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Cloning the nuodb-operator GitHub repo..."
git clone --branch $NUOOPGITBRANCH https://github.com/nuodb/nuodb-operator.git
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Creating nuodb-operator service account..."
kubectl create -f nuodb-operator/deploy/service_account.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Creating nuodb-operator role..."
kubectl create -f nuodb-operator/deploy/role.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Creating nuodb-operator role binding..."
kubectl create -f nuodb-operator/deploy/role_binding.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Creating nuodb-op-admin cluster role binding..."
kubectl create clusterrolebinding nuodb-op-admin --clusterrole cluster-admin --serviceaccount=nuodb:nuodb-operator
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Create secret for registry credentials for Quay.io..."
kubectl create secret generic regcred --from-file=.dockerconfigjson=$HOME/.docker/config.json --type=kubernetes.io/dockerconfigjson
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Creating nuodbadmin CRD..."
kubectl create -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbadmin_crd.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Creating nuodb CRD..."
kubectl create -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodb_crd.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Creating nuodbycsbwl CRD..."
kubectl create -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbycsbwl_crd.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Creating nuodbinsightsserver CRD..."
kubectl create -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbinsightsserver_crd.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi


if [ $openshift -eq 1 ]; then
  echo ""
  echo "Setting oc adm policy..."
  oc adm policy add-scc-to-user privileged -n elastic-system -z elastic-operator
  retval=$?
  if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi
  oc adm policy add-scc-to-user privileged -n nuodb -z elastic-operator
  retval=$?
  if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi
  oc adm policy add-scc-to-user privileged -z insights-server-release-logstash
  retval=$?
  if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi
fi

echo ""
echo "Deploy the NuoDB Operator..."
sed "s/REPLACE_IMAGE/quay.io\/nuodb\/$NUOOPIMAGE/" nuodb-operator/deploy/operator.yaml > nuodb-operator/deploy/operator-test.yaml
kubectl create -f nuodb-operator/deploy/operator-test.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
date
echo "Create NuoDB Admin CR..."
kubectl create -f nuodb-operator/test/deploy/crs/nuodb_v2alpha1_nuodbadmin_test_cr.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo -n "Checking Admin statefulsets..."
while [[ $(kubectl get pods -l app=admin -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]
do
  ((i++))
  if [[ "$i" == '36' ]]; then
    echo "ERROR: Timeout waiting for Admin StatefulSet."
    echo "$0: FAIL"
    exit 1
  fi
  sleep 5
  echo -n '.'
done
echo ""
echo -n "Checking nuoadmin statefulset..."
kubectl wait --namespace=nuodb --for=condition=ready pod --timeout=60s -l statefulset.kubernetes.io/pod-name=admin-0
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
date
echo "Patching nuodbadmin/nuoadmin CR to enable Hosted NuoDB Insights..."
kubectl patch nuodbadmin nuoadmin --type='json' -p='[{"op": "replace", "path": "/spec/insightsEnabled", "value":true}]'
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo -n "Checking nuodb-insights pod..."
while [[ $(kubectl get pods -l insights=hosted -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]
do
  ((i++))
  if [[ "$i" == '36' ]]; then
    echo "ERROR: Timeout waiting for nuodb-insights Pod."
    echo "$0: FAIL"
    exit 1
  fi
  sleep 5
  echo -n '.'
done
echo "nuodb-insights pod found"

echo ""
date
echo "Patching nuodbadmin/nuoadmin CR to disable Hosted NuoDB Insights..."
kubectl patch nuodbadmin nuoadmin --type='json' -p='[{"op": "replace", "path": "/spec/insightsEnabled", "value":false}]'
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo -n "Waiting for nuodb-insights pod to be deleted..."
while [[ $(kubectl get pods -l insights=hosted) ]]
do
  ((i++))
  if [[ "$i" == '36' ]]; then
    echo "ERROR: Timeout waiting for nuodb-insights Pod."
    echo "$0: FAIL"
    exit 1
  fi
  sleep 5
  echo -n '.'
done
echo "nuodb-insights pod deleted"


echo ""
date
echo "Create NuoDB test1 CR..."
kubectl create -f nuodb-operator/test/deploy/crs/nuodb_v2alpha1_nuodb_test1_cr.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
date
echo "Create NuoDB test2 CR..."
kubectl create -f nuodb-operator/test/deploy/crs/nuodb_v2alpha1_nuodb_test2_cr.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi


echo ""
echo "Create NuoDB Insights Server CR..."
kubectl create -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbinsightsserver_cr.yaml 
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo -n "Checking SM statefulsets..."
while [[ $(kubectl get pods -l app=sm -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True True" ]]
do
  ((i++))
  if [[ "$i" == '36' ]]; then
    echo "ERROR: Timeout waiting for SM."
    echo "$0: FAIL"
    exit 1
  fi
  sleep 5
  echo -n '.'
done
echo ""
echo -n "Checking nuodb-test1 SM statefulset..."
kubectl wait --namespace=nuodb --for=condition=ready pod --timeout=180s -l statefulset.kubernetes.io/pod-name=nuodb-test1-sm-0
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi
echo ""
echo -n "Checking nuodb-test2 SM statefulset..."
kubectl wait --namespace=nuodb --for=condition=ready pod --timeout=180s -l statefulset.kubernetes.io/pod-name=nuodb-test2-sm-0
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Checking nuodb-test1 TE deployment..."
kubectl rollout status deployment nuodb-test1-te
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Checking nuodb-test2 TE deployment..."
kubectl rollout status deployment nuodb-test2-te
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo -n "Checking Admin statefulset..."
while [[ $(kubectl get pods -l app=admin -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]
do
  ((i++))
  if [[ "$i" == '36' ]]; then
    echo "ERROR: Timeout waiting for Admin."
    echo "$0: FAIL"
    exit 1
  fi
  sleep 5
  echo -n '.'
done
echo ""
kubectl wait --namespace=nuodb --for=condition=ready pod --timeout=120s -l statefulset.kubernetes.io/pod-name=admin-0
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "Create NuoDB YCSB Workload CR..."
kubectl create -f nuodb-operator/test/deploy/crs/nuodb_v2alpha1_nuodbycsbwl_test1_cr.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
date
echo -n "Waiting for logstash to become ready..."
i=0
while [[ $(kubectl get pods -l app=logstash -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True" ]]
do
  ((i++))
  if [[ "$i" == '46' ]]; then
    echo "ERROR: Timeout waiting for Logstash."
    echo "$0: FAIL"
    exit 1
  fi
  sleep 5
  echo -n '.'
done
echo ""
echo "Logstash is ready."

echo ""
date
echo "Starting insights-client..."
kubectl create -f nuodb-operator/build/etc/insights-server/insights-client.yaml
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi
kubectl -n nuodb wait --for condition=Ready --timeout=30s pod/insights-client
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi
date
kubectl get pods -n nuodb

echo ""
echo "nuocmd show domain..."
kubectl exec -it admin-0 /bin/bash nuocmd show domain -n nuodb
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "nuocmd check database --db-name test1..."
kubectl -n nuodb exec -it admin-0 -- /bin/bash -c "nuocmd check database --db-name test1"
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

echo ""
echo "nuocmd check servers..."
kubectl -n nuodb exec -it admin-0 -- /bin/bash -c "nuocmd check servers --timeout 30"
retval=$?
if [ $retval -ne 0 ]; then echo "$0: FAIL"; exit 1; fi

export ES_PASSWORD=$(kubectl get secret insights-escluster-es-elastic-user -o go-template='{{.data.elastic | base64decode }}')
kubectl get secret insights-escluster-es-http-certs-public -o go-template='{{index .data "tls.crt" | base64decode }}' > es.cert

echo ""
date
echo "$0: PASS"
