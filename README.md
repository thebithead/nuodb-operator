# The NuoDB Operator
A Kubernetes Operator written in Golang that automates the packaging, provisioning, and managing of operational tasks for Kubernetes containerized applications. By default the NuoDB Kubernetes Operator deploys the NuoDB with Community Edition (CE) capability in the following tested and verified Kubernetes distributions:

* Red Hat OpenShift 3.11 or 4.x
* Google Cloud Platform (GCP) - GCE nodes running GKE managed Kubernetes
* Google Cloud Platform (GCP) - GCE nodes running open source Kubernetes
* Amazon Web Services (AWS)   - EC2 nodes running EKS managed Kubernetes
* Amazon Web Services (AWS)   - EC2 nodes running open source kubernetes

The NuoDB Operator also supports deploying NuoDB with either ephemeral or persistent storage options with configurations to run NuoDB Insights, a visual database monitoring Web UI, and start a sample SQL application (ycsb) to quickly generate a user-configurable SQL workload against the database.

## About the NuoDB Community Edition Capability
The NuoDB Community Edition (CE) capability is a full featured, but limits the database to one Storage Manager (SM) and three Transaction Engine (TE) processes. The Community Edition is free of charge and allows you to self-evaluate NuoDB at your own pace. The NuoDB Community Edition (CE) will allow first time users to experience all the benefits and value points of NuoDB including: 

* Ease of scale-out to meet changing application throughput requirements
* Continuous availability even in the event of common network, hardware, and software failures
* NuoDB database and workload visual monitoring with NuoDB Insights
* ANSI SQL
* ACID transactions

To effectively evaluate the NuoDB Community Edition (CE) we recommend creating a Kubernetes cluster of at least three nodes. To fully demonstrate transactional scale-out and database continuous availability we recommend four or five nodes.

As you proceed through the steps outlined on this page, we would like your self-guided NuoDB CE operator and database experience in Kubernetes to be a positive one! Please reach out to us at support@nuodb.com with any questions or comments you may have. We would be glad to learn more about your specific use case and provide assistance if needed!

To trial or run a PoC of the NuoDB Enterprise Edition (EE) which also allows users to scale the Storage Manager (SM) database process, contact NuoDB Sales at sales@nuodb.com for a PoC time-based enterprise edition license. For more information about NuoDB, see the [NuoDB Website](https://www.nuodb.com).

## NuoDB Operator Page Outline
This page is organized in the following sections:

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Installation Prerequisites](#Installation-Prerequisites)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Install the NuoDB Operator](#Install-the-NuoDB-Operator)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Deploy the NuoDB Database](#Deploy-the-NuoDB-Database)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Deploy the NuoDB Insights Visual Monitor](#Deploy-the-NuoDB-Insights-Visual-Monitor)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Launch a Sample SQL Workload](#Launch-a-Sample-SQL-Workload)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Sample NuoDB Features and Benefits Evaluation Steps](#Sample-NuoDB-Features-and-Benefits-Evaluation-Steps)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Delete the NuoDB Database](#Delete-the-NuoDB-Database)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Delete the NuoDB Operator](#Delete-the-NuoDB-Operator)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Optional Database Parameters](#Optional-Database-Parameters)



## Installation Prerequisites

_**Note:** The instructions on this page use the Kubernetes&ensp;`kubectl` command (for portability reasons across Kubernetes environments). For enviornments, the&ensp;`kubectl` command is an alias that points to the OpenShift client program&ensp;`oc`._


### 1. Create environment variables

```
export OPERATOR_NAMESPACE=nuodb
export STORAGE_NODE=yourStorageNodeDNSName
export NUODB_OPERATOR_VERSION=2.0.1           --confirm you set the correction NuoDB Operator version here.
```
### 2. Clone a copy of the NuoDB Operator from Github
In your home or working directory, run:

&ensp; `git clone https://github.com/nuodb/nuodb-operator`

### 3. Log into your Kubernetes cluster

&ensp; `Example: kubectl login -u system:admin`

### 4. Create the "nuodb" project/namespace (if not already created)

&ensp; `kubectl create namespace $OPERATOR_NAMESPACE`

### 5. Set the project/namespace value

&ensp; `kubectl project $OPERATOR_NAMESPACE`

### 6. Set container storage pre-requisites

Amazon EBS storage (storageclass gp2) is the default storage class for both NuoDB Admin and Storage Manager (SM) pods. 
If you would like to change the default, please see below: 

#### FOR ON-PREM local storage (using Hostpath): 
Set the local storage permissions on each cluster node to enable hosting storage for either the Admin or the Storage Manager (SM) pods.
**Note:** When using the local disk storage option only 1 Admin pod is supported.

```
sudo mkdir -p /mnt/local-storage/disk0
sudo chmod -R 777 /mnt/local-storage/
sudo chcon -R unconfined_u:object_r:svirt_sandbox_file_t:s0 /mnt/local-storage
sudo chown -R root:root /mnt/local-storage
```
Create the storage class "local-disk" and persistent volume

&ensp; `kubectl create -f nuodb-operator/deploy/local-disk-class.yaml`

Set the storage class values in cr.yaml

```
adminStorageClass: local-disk
smStorageClass: local-disk
```

#### FOR Amazon AWS: Set the storage class values in cr.yaml

```
adminStorageClass: gp2
smStorageClass: gp2
```

#### FOR Google GCP: Set the storage class values in cr.yaml 

```
adminStorageClass: standard
smStorageClass: standard
```
#### When using 3rd-party container-native storage(e.g. OpenEBS,LinStor,Portworx,etc.): Set the storage class values in cr.yaml 

```
adminStorageClass: <3rd-party storageClassName>
smStorageClass: <3rd-party storageClassName>
```

### 7. Cluster Node Labeling
Label the cluster nodes you want to run NuoDB pods.

&ensp; `kubectl  label node <node name> nuodb.com/zone=nuodb`

_**Note:** The label value, in this example "nuodb", can be any value._

Next, label one of these nodes as your storage node. This is the node that will host your NouDB Storage Manager (SM) pod and is where you database persistent storage will reside. Ensure there is sufficient disk space. To create this label run:

&ensp; `kubectl  label node $STORAGE_NODE nuodb.com/node-type=storage`

Once your cluster nodes are labeled for NuoDB use, run the following&ensp; `kubectl get nodes` command to confirm nodes are labeled prperly. The display output should look similar to the below
```
kubectl get nodes -l nuodb.com/zone -L nuodb.com/zone,nuodb.com/node-type
NAME                           STATUS   ROLES    AGE   VERSION             ZONE    NODE-TYPE
ip-10-0-141-113.ec2.internal   Ready    worker   15d   v1.13.4+cb455d664   nuodb   storage
ip-10-0-152-147.ec2.internal   Ready    worker   15d   v1.13.4+cb455d664   nuodb   
ip-10-0-162-73.ec2.internal    Ready    worker   15d   v1.13.4+cb455d664   nuodb   
ip-10-0-184-233.ec2.internal   Ready    worker   15d   v1.13.4+cb455d664   nuodb   
ip-10-0-206-8.ec2.internal     Ready    worker   15d   v1.13.4+cb455d664   nuodb 
```

### 8. Apply a NuoDB license file

Each time a NuoDB Admin pod starts it will load a Kubernetes configmap that contains the current NuoDB license level information and places its contents in the /etc/nuodb/nuodb.lic file. When a request is made to either start a NuoDB Transaction Engine (TE) or Storage Manager (SM) process, the NuoDB Admin will check the license file contents to ensure the process request is authorized.

To apply a NuoDB Communnity Edtion (CE) license file, run

&ensp; `kubectl create configmap nuodb-lic-configmap -n $OPERATOR_NAMESPACE --from-literal=nuodb.lic=""`

To apply a NuoDB Enterprise Edtion (EE) license file, obtain your license file from your NuoDB Sales or Support representative and copy the file to&ensp;`nuodb.lic`, then run

&ensp; `kubectl create configmap nuodb-lic-configmap -n $OPERATOR_NAMESPACE --from-file=nuodb.lic`

**Note:** The filename specified must be nuodb.lic

### 9. If using the Red Hat OpenShift 

#### To permit the pulling of the container and operator images, create the Kubernetes image pull secret
**Note:** If using Quay.io to pull the NuoDB Operator image, a login to quay.io and a Kubernetes secret is not required because the NuoDB Quay.io repository is public. For example, to pull the image from quay.io, run at the command prompt, docker pull quay.io/nuodb/nuodb-operator

This secret will be used to pull the NuoDB Operator and NuoDB container images from the  Red Hat Container
Catalog (RHCC). Enter your Red Hat login credentials for the --docker-username and --docker-password values.

```
kubectl  create secret docker-registry pull-secret \
   --docker-username='yourUserName' --docker-password='yourPassword' \
   --docker-email='yourEmailAddr'  --docker-server='registry.connect.redhat.com'
 ```

#### Disable Linux Transparent Huge Pages (THP). Run the following required command to create a security context constraint which will allow the Operator to disable THP during Operator deployment.
```
kubectl create -n $OPERATOR_NAMESPACE -f thp-scc.yaml
```

## Install the NuoDB Operator

To install the NuoDB Operator into your Kubernetes cluster, follow the steps indicated for the appropriate Kubernetes Distribtion you are using.

### Red Hat OpenShift v4.x 

In OpenShift 4.x, the NuoDB Operator is available to install directly from the OpenShift OperatorHub, an integrated service catalog, accessible from within the OpenShift 4 Web UI which creates a seamless - single click experience - that allows users to install the NuoDB Operator from catalog-to-cluster in seconds.

1. Select &ensp;`Projects` from the OpenShift 4 left toolbar and click the &ensp;`NuoDB` project to make
   it your current project.
2. Select the &ensp;`OperatorHub` under the &ensp;`Catalog` section in the OCP 4 left toolbar.
3. Select the &ensp;`Database` filter and scroll down to the NuoDB Application tile and click the tile.
4. In the right-hand corner of the NuoDB Operator page, click the &ensp;`Install` button.
5. On the "Create Operator Subscription" page, select the radio group option "A specific namespace on the cluster"
   and enter the project/namespace in the pull-down field that you would like to install the NuoDB Operator,
   then select &ensp;`Subscribe` to subscribe to the NuoDB Operator.
6. In less than a minute, on the page that displays should indicate the NuoDB Operator has been
   installed, see "1 installed" message.
7. To verify the NuoDB Operator installed correctly, select &ensp;`Installed Operators` from the left
   toolbar. The STATUS column should show "Install Succeeded".
8. Select &ensp;`Status` under the &ensp;`Projects` on the left toolbar to view your running Operator.

### Red Hat OpenShift v3.11 --or-- Open soure / GKE / EKS Kubernetes

#### If not already installed, then install the Operator Lifecycle Manager (OLM)
```
kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.10.1/crds.yaml
kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.10.1/olm.yaml
```
#### If not already installed, then install the Operator Marketplace (OPTIONAL)

Clone the Operator marketplace repository
```
https://github.com/operator-framework/operator-marketplace
```

Note: If you experience the following error when running the catalogSource.yaml file in the next section, then you can install the Operator Marketplace to resolve this error. However, the error can also be ignored. The NuoDB Operator will install successfully without the Operator Marketplace.
```
error: unable to recognize "catalogSource.yaml": no matches for kind "OperatorSource" in version "operators.coreos.com/v1"
```

#### Deploy the NuoDB Operator
```
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/role.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/role_binding.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/service_account.yaml
kubectl create -n $OPERATOR_NAMESPACE -f https://raw.githubusercontent.com/nuodb/nuodb-operator/master/deploy/cluster_role.yaml
kubectl create -n $OPERATOR_NAMESPACE -f https://raw.githubusercontent.com/nuodb/nuodb-operator/master/deploy/cluster_role_binding.yaml

## for Red Hat OpenShift only
oc adm policy add-scc-to-user privileged system:serviceaccount:nuodb:nuodb-operator
oc adm policy add-scc-to-user privileged system:serviceaccount:elastic-system:elastic-operator
oc adm policy add-scc-to-user privileged system:serviceaccount:nuodb:insights-server-release-logstash

## add NuoDB, Insights, and ycsb sample SQL app CRDs
kubectl create -f nuodb-operator/deploy/crds/nuodb_v1alpha1_nuodb_crd.yaml
kubectl create -f nuodb-operator/deploy/crds/nuodb_v1alpha1_nuodbycsbwl_crd.yaml
kubectl create -f nuodb-operator/deploy/crds/nuodb_v1alpha1_nuodbinsightsserver_crd.yaml

sed "s/placeholder/$OPERATOR_NAMESPACE/" olm-catalog/nuodb-operator/$NUODB_OPERATOR_VERSION/nuodb.v$NUODB_OPERATOR_VERSION.clusterserviceversion.yaml > nuodb-csv.yaml

# To replace quay.io as the default location to pull the NuoDB Operator image, follow these examples
# To pull from the Red Hat Container Catalog, run
   sed "s/quay.io/registry.connect.redhat.com/" nuodb-csv.yaml > nuodb-csv-rhcc.yaml
# To pull from the Google Marketplace, run
   sed "s/quay.io/marketplace.gcr.io/"          nuodb-csv.yaml > nuodb-csv-gcp.yaml
# To pull from the AWS Marketplace, 
# replace in the nuodb-csv.yaml file the two image references wtih 709373726912.dkr.ecr.us-east-1.amazonaws.com/d893f8e5-fe12-4e43-b792-8cb98ffc11c0/cg-1180980994/quay.io/nuodb/nuodb-operator:0.0.5-latest

# If appliable, copy your new nuodb-csv-xxx.yaml file to nuodb-csv.yaml and run,
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-csv.yaml

# Check deployment rollout status every 5 seconds (max 10 minutes) until complete.
ATTEMPTS=0
ROLLOUT_STATUS_CMD="kubectl rollout status deployment/nuodb-operator -n $OPERATOR_NAMESPACE"
until $ROLLOUT_STATUS_CMD || [ $ATTEMPTS -eq 60 ]; do
  $ROLLOUT_STATUS_CMD
  ATTEMPTS=$((attempts + 1))
  echo ""
  kubectl get pods -n nuodb
  sleep 5
done
```

## Deploy the NuoDB Database

Below is a sample of how to deploy a NuoDB database using "on-cluster" NuoDB Insight visual monitoring and start a sample SQL application
```
# To deploy the NuoDB database into your Kubernetes cluster, first make a local copy of the NuoDB cr yaml files
cp nuodb-operator/deploy/crds/nuodb_v1alpha1_nuodb_cr.yaml                 nuodb-cr.yaml
cp nuodb-operator/deploy/crds/nuodb_v1alpha1_nuodbinsightsserver_cr.yaml   nuodb-insights-cr.yaml
cp nuodb-operator/deploy/crds/nuodb_v1alpha1_nuodbycsbwl_cr.yaml           nuodb-ycsbwl_cr.yaml

# Modify / customize your NuoDB cr yaml files and run, (see samples below in next section)
kubectl create -f nuodb-cr.yaml
kubectl create -f nuodb-insights-cr.yaml
kubectl create -f nuodb-ycsb-cr.yaml

#Wait for nuodb to be logstash instance to be ready
# Check deployment rollout status every 10 seconds (max 10 minutes) until complete.
ATTEMPTS=0
ROLLOUT_STATUS_CMD="kubectl rollout status sts/insights-server-release-logstash -n $OPERATOR_NAMESPACE"
until $ROLLOUT_STATUS_CMD || [ $ATTEMPTS -eq 60 ]; do
  $ROLLOUT_STATUS_CMD
  ATTEMPTS=$((attempts + 1))
  echo ""
  kubectl get pods -n $OPERATOR_NAMESPACE
  sleep 5
done

# create the Insights client
kubectl create -f nuodb-operator/build/etc/insights-server/insights-client.yaml

echo ""
echo "Open Networking/Route panel in your Kubernetes dashboard to obtain your NuoDB Insight's dashboard URL."
echo ""
echo "https://$(kubectl get route grafana-route --output=jsonpath={.spec.host})//d/000000002/system-overview?orgId=1&refresh=10s"

 ```

### Sample nuodb-cr.yaml deployment files

The nuodb-operator/deploy directory includes sample Custom Resources to deploy NuoDB:

&ensp; `cr-ephemeral.yaml` deploys NuoDB CE domain without a persistent storage volume by setting storageMode to "ephemeral".

&ensp; `cr-persistent-insights-enabled.yaml` deploys NuoDB CE domain using persistent storage and has NuoDB Insights enabled.

Optionally, you can add any of these below parameters values to your own `nuodb-cr.yaml` to customize your database. Each are described in the Optional Database Parameter section. A sample nuodb-ycsb-cr.yaml is also provided. The nuodb-insights-cr.yaml normally does not require modification.
```
spec:
  replicaCount: 1
  storageMode: persistent
  insightsEnabled: true
  adminCount: 3
  adminStorageSize: 2G
  adminStorageClass: local-disk
  dbName: test
  dbUser: dba
  dbPassword: secret
  smCount: 1
  smMemory: 4
  smCpu: 2
  smStorageSize: 20G
  smStorageClass: local-disk
  engineOptions: ""
  teCount: 1
  teMemory: 4
  teCpu: 2
  container: nuodb/nuodb-ce:latest
```

**Note:** We recommend replacing the database password `dbPassword` value 'secret' wtih one of your choice. Also, it's common to configure the image pull source locations by replacing the default values for the `ycsbContainer` and `container` parameters with values that match your deployment type. See section&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Optional Database Parameters](#Optional-Database-Parameters) for working samples.

### Sample nuodb-ycsb-cr.yaml deployment files
```
ycsbLoadName: ycsb-load
  ycsbWorkload: b
  ycsbLbPolicy: ""
  ycsbNoOfProcesses: 5
  ycsbNoOfRows: 10000
  ycsbNoOfIterations: 0
  ycsbOpsPerIteration: 10000
  ycsbMaxDelay: 240000
  ycsbDbSchema: User1
  ycsbContainer: nuodb/ycsb:latest
```


### Deploy the NuoDB Insights Visual Monitor

Optionally deploy the NuoDB Insights visual monitoring tool **(recommended)**. Insights is a powerful database monitoring tool that can greatly aid in visualizing database workload and resource consumption. For more information about the benefits of Insights, please refer to the [NuoDB Insights](https://www.nuodb.com/product/insights) Webpage.
        
> Insights is also part of NuoDB Services and Support in order to service our customers better and more efficently and is
      subject to our Terms of Use and Privacy Policy.
      [Terms of Use](https://www.nuodb.com/terms-use) and [Privacy Policy](https://www.nuodb.com/privacy-policy)
      Insights collects anonymized data about your NuoDB implementation, and use,
      including system information, configuration, response times, load averages,
      usage statistics, and user activity logs ("Usage Information").  Usage
      Information does not include any personally identifiable information ("PII"),
      but may include some aggregated and anonymized information derived from data
      that may be considered PII in some contexts (e.g., user locations or IP
      addresses).
      NuoDB uses Usage Information to monitor, analyze and improve the performance
      and reliability of our Services, and to contribute to analytical models used by
      NuoDB.  Usage Information is not shared with any third parties.  Insights also
      includes a user dashboard that allows administrators to view the performance of
      your NuoDB implementation.
      If you agree to these terms, type "true" in your cr.yaml file for the field
      *insightsEnabled* to "Opt In" and enable NuoDB Insights. Any other value than "true"
      results in Opting out. Insights can also be enabled at a later time if you choose.

After deploying your NuoDB database, if you optionally chose to install NuoDB Insights, you can find your NuoDB Insights SubcriberID by locating the "nuodb-insights" pod, go to the Logs tab, and find the line that indicates your Subscriber ID. 
```
Insights Subscriber ID: yourSubID#
```

**NOTE:** Obtaining your Subscriber ID is only required if you are using the NuoDB hosted Insights portal. If you are deploying on-cluster NuoDB Insights then your URL to access your locally deployed Insight's Web UI can be obtained by running,
```
https://$(kubectl get route grafana-route --output=jsonpath={.spec.host})//d/000000002/system-overview?orgId=1&refresh=10s
```

**Usage note when using open source Kubernetes only:** A current Kubernetes Web UI issue doesn't allow users to retrieve their Insights Subscription ID using the K8s open source WebUI by reviewing the container log, instead run
```
kubectl logs nuodb-insights -n nuodb -c insights
```
To connect to NuoDB Insights, open a Web browser using the following URL

https://insights.nuodb.com/yourSubID#

### Check the status of NuoDB Insights visual monitoring tool
If you enabled NuoDB Insights (highly recommended) you can confirm it's run status by running:

&ensp; `oc exec -it nuodb-insights -c insights -- nuoca check insights`


## Launch a Sample SQL Workload

The NuoDB Operator includes a sample SQL appplication that will allow you to get started quickly running SQL statements against your NuoDB database. The sample workload uses YCSB (the Yahoo Cloud Servicing Benchmark). The cr.yaml includes YCSB parameters that will allow you to configure the SQL workload to your preferences.

To start a SQL Workload (if your nuodb-ycsb-cr.yaml isn't configured to start one by default) locate the ycsb Replication Controller in your Kubernetes dashboard and scale it to your desired number of pods to create your desired SQL application workload. Once the YCSB application is running the resulting SQL workload will be viewable from the NuoDB Insights visual monitoring WebUI.

## Sample NuoDB Features and Benefits Evaluation Steps

Once your NuoDB database is running, here are a few steps to try out to quickly realize the benefits of running a NuoDB SQL database

* Demonstrate Transactional Scale-out
* Demonstrate Continuous Availability
* Demonstrate Visual Monitoring Using NuoDB Insights

The following videos provide a walk-thru of each feature area mentioned above. These videos show NuoDB running in Red Hat OpenShift, but the sample steps can be performed in any NuoDB supported Kubernetes environment.

   [NuoDB in OpenShift v3.11 video](https://www.youtube.com/playlist?list=PLQJ_WGDxAYCG340ODaOACaG9KIG6IQBYL)

   [NuoDB in OpenShift  v4.x video](https://www.youtube.com/playlist?list=PLQJ_WGDxAYCEU7Ae-b7shbwE8bn_iJDwM)


## Delete the NuoDB database

kubectl delete -n $OPERATOR_NAMESPACE nuodb nuodb
Next, delete the nuodb database finalizer by running this command, remove the finalizer line under "Finalizer:", and run the final nuodb delete commmand
```
kubectl edit nuodb nuodb
kubectl delete nuodb nuodb

kubectl delete pod/insights-client
kubectl delete nuodbinsightsservers/insightsserver
kubectl delete -f nuodb-golang-operator/deploy/crds/nuodb_v1alpha1_nuodbycsbwl_crd.yaml
kubectl delete -f nuodb-golang-operator/deploy/crds/nuodb_v1alpha1_nuodbinsightsserver_crd.yaml

Delete the NuoDB persistent storage volumes claims
kubectl delete -n $OPERATOR_NAMESPACE pvc --all 
```

If the local-disk storage class was used, then delete the NuoDB Storage Manager(SM) disk storage and storage class
```
ssh -i ~/Documents/cluster.pem $JUMP_HOST
ssh -i ~/.ssh/cluster.pem core@ip-n-n-n-n.ec2.internal  'rm -rf /mnt/local-storage/disk0/*'

kubectl delete -f local-disk-class.yaml
```

## Delete the NuoDB Operator

### Red Hat OpenShift v4.x
From the OpenShift WebUI, locate the OperatorHub under the Catalog left-bar selection. Select the NuoDB Operator and click the Uninstall button.

### Red Hat OpenShift v3.11 --or-- Open source / GKE / EKS Kubernetes

Run the following commands
```
kubectl delete -n $OPERATOR_NAMESPACE configmap nuodb-lic-configmap

kubectl delete pod/insights-client
kubectl delete nuodbinsightsservers/insightsserver
kubectl delete nuodbs/nuodb-db
kubectl delete nuodbycsbwls/nuodbycsbwl
kubectl delete pvc --all 
kubectl delete pv --all

kubectl delete -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/role.yaml
kubectl delete -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/role_binding.yaml
kubectl delete -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/service_account.yaml

kubectl delete -n $OPERATOR_NAMESPACE -f https://raw.githubusercontent.com/nuodb/nuodb-operator/master/deploy/cluster_role.yaml
kubectl delete -n $OPERATOR_NAMESPACE -f https://raw.githubusercontent.com/nuodb/nuodb-operator/master/deploy/cluster_role_binding.yaml

kubectl delete -f nuodb-golang-operator/deploy/crds/nuodb_v1alpha1_nuodb_crd.yaml
kubectl delete -f nuodb-golang-operator/deploy/crds/nuodb_v1alpha1_nuodbycsbwl_crd.yaml
kubectl delete -f nuodb-golang-operator/deploy/crds/nuodb_v1alpha1_nuodbinsightsserver_crd.yaml

-- For OPENSHIFT only, delete the thp security context constraint
kubectl delete scc thp-scc

Delete the NuoDB project/namespace
```
kubectl delete namespace $OPERATOR_NAMESPACE
```

Verify cleanup
```
kubectl get grafanas
kubectl get grafanadatasources
kubectl get grafanadashboards
kubectl get kibanas
kubectl get elasticsearches
kubectl get sa/grafana-operator
kubectl get secrets | grep grafana-operator 
kubectl get rolebindings/grafana-operator
kubectl get role/grafana-operator
```


## Optional Database Parameters

**storageMode** - Run NuoDB CE using a persistent, local, disk volume "persistent" or volatile storage "ephemeral". Must be set to one of those values.

&ensp; `storageMode: persistent`


**insightsEnabled** - Use to control NuoDB Insights Opt In. NuoDB Insights provides database monitoring and visualization. Set to "true" to activate or "false" to deactivate.

&ensp; `insightsEnabled: false`


**adminCount** - Number of admin service pods. Requires 1 server node available for each Admin Service

&ensp; `adminCount: 1`


**adminStorageSize** - Admin service log volume size (GB)

&ensp; `adminStorageSize: 5G`


**adminStorageClass** - Admin persistent storage class name

&ensp; `adminStorageClass: glusterfs-storage`


**dbName** - NuoDB Database name. must consist of lowercase alphanumeric characters '[a-z0-9]+' 

&ensp; `dbName: test`


**dbUser** - Name of Database user

&ensp; `dbUser: dba`


**dbPassword** - Database password

&ensp; `dbPassword: secret`


**smMemory** - SM memory (in GB)

&ensp; `smMemory: 2`


**smCpu** - SM CPU cores to request

&ensp; `smCpu: 1`


**smStorageSize** - Storage manager (SM) volume size (GB)

&ensp; `smStorageSize: 20G`


**smStorageClass** - SM persistent storage class name

&ensp; `smStorageClass: local-disk`


**engineOptions** - Additional "nuodb" engine options Format: …

&ensp; `engineOptions: ""`


**teCount** - Number of transaction engines (TE) nodes. Limit is 3 in CE version of NuoDB

&ensp; `teCount: 1`


**teMemory** - TE memory (in GB)

&ensp; `teMemory: 2`


**teCpu** - TE CPU cores to request

&ensp; `teCpu: 1`


**ycsbLoadName** - YCSB workload pod name

&ensp; `ycsbLoadName: ycsb-load`


**ycsbWorkload** - Sample SQL activity workload. Valid values are a-f. Each letter determines a different mix of read and update workload percentage generated. a= 50/50, b=95/5, c=100 read. Refer to YCSB documentation for more detail.

&ensp; `ycsbWorkload: b`


**ycsbLbPolicy** - YCSB load-balancer policy. Name of an existing load-balancer policy, that has already been created using the 'nuocmd set load-balancer' command.

&ensp; `ycsbLbPolicy: ""`


**ycsbNoOfProcesses** - Number of YCSB processes. Number of concurrent YCSB processes that will be started in each YCSB pod. Each YCSB process makes a connection to the Database.

&ensp; `ycsbNoOfProcesses: 2`


**ycsbNoOfRows** - YCSB number of initial rows in table

&ensp; `ycsbNoOfRows: 10000`


**ycsbNoOfIterations** - YCSB number of iterations

&ensp; `ycsbNoOfIterations: 0`


**ycsbOpsPerIteration** - Number of YCSB SQL operations to perform in each iteration. This value controls the number of SQL operations performed in each benchmark iteration. Increasing this value increases the run-time of each iteration, and also reduces the frequency at which new connections are made during the sample workload run period.

&ensp; `ycsbOpsPerIteration: 10000`


**ycsbMaxDelay** - YCSB maximum workload delay in milliseconds (Default is 4 minutes)

&ensp; `ycsbMaxDelay: 240000`


**ycsbDbSchema** - YCSB Database schema. Default schema to use to resolve tables, views, etc.

&ensp; `ycsbDbSchema: User1`


**apiServer** - Load balancer service URL. hostname:port (or LB address) for nuocmd and nuodocker process to connect to.

&ensp; `apiServer: https://domain:8888`


**container** - NuoDB fully qualified image name (FQIN) for the Docker image to use

Below are examples that pull the NuoDB container image from Red Hat (RHCC), Google Cloud Platform Marketplace, AWS Marketplace, and DockerHub.

```
container: registry.connect.redhat.com/nuodb/nuodb-ce:latest
container: marketplace.gcr.io/nuodb/nuodb:latest
container: 709373726912.dkr.ecr.us-east-1.amazonaws.com/d893f8e5-fe12-4e43-b792-8cb98ffc11c0/cg-1180980994/docker.io/nuodb/nuodb-ce:0.0.5-latest
container: nuodb/nuodb-ce:latest
```

**ycsbContainer** - YCSB fully qualified image name (FQIN) for the ycsb docker image to use. See examples below pulling the image from dockerhub and the AWS Marketplace.

```
ycsbContainer: nuodb/ycsb:latest
ycsbContainer: 709373726912.dkr.ecr.us-east-1.amazonaws.com/d893f8e5-fe12-4e43-b792-8cb98ffc11c0/cg-1180980994/docker.io/nuodb/ycsb:0.0.5-latest
```
