<img src="https://www.nuodb.com/sites/all/themes/nuodb/logo.svg" data-canonical-src="https://www.nuodb.com/sites/all/themes/nuodb/logo.svg" width="200" height="200" />

# The NuoDB Operator

[![Build Status](https://travis-ci.org/nuodb/nuodb-operator.svg?branch=master)](https://travis-ci.org/nuodb/nuodb-operator)

A Kubernetes Operator written in Golang that automates the packaging, provisioning, and managing of operational tasks for Kubernetes containerized applications. By default the NuoDB Kubernetes Operator deploys the NuoDB with Community Edition (CE) capability in the following tested and verified Kubernetes distributions:

* Red Hat OpenShift 3.11, 4.x
  * On-prem or OpenShift supported public cloud platforms
* Google Cloud Platform (GCP)
  * GKE managed Kubernetes
  * Anthos GKE (on-prem) managed Kubernetes
  * Open source Kubernetes
* Amazon Web Services (AWS)
  * EKS managed Kubernetes
  * Open source kubernetes
* Rancher Kubernetes Manager
  * Rancher RKE and Rancher supported Kubernetes (e.g. EKS, AKS) on Rancher supported cloud platforms

**Note:** The NuoDB Operator is ideal for development, test and for product evaluation purposes in single cluster Kubernetes environments. **The NuoDB Operator is not yet available for production use.** Coming soon in 1H'20 is automated LEVEL 2 (rolling upgrade) and LEVEL 3 (backup and recovery) operational support. Multi-cluster support is also on the roadmap.

The NuoDB Operator also supports deploying NuoDB with either ephemeral or persistent storage options with configurations to run NuoDB Insights, a visual database monitoring Web UI, and start a sample application (ycsb) to quickly generate a user-configurable SQL workload against the database.

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

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Configure NuoDB Insights Visual Monitor](#Configure-NuoDB-Insights-Visual-Monitor)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Install the NuoDB Operator](#Install-the-NuoDB-Operator)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Deploy the NuoDB Database](#Deploy-the-NuoDB-Database)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Connect to your Database using nuosql](#Connect-to-your-Database-using-nuosql)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Launch a Sample SQL Workload](#Launch-a-Sample-SQL-Workload)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[NuoDB Features and Benefits Evaluation Steps](#NuoDB-Features-and-Benefits-Evaluation-Steps)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Delete the NuoDB Database](#Delete-the-NuoDB-Database)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Delete the NuoDB Operator](#Delete-the-NuoDB-Operator)

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[Optional Database Parameters](#Optional-Database-Parameters)



## Installation Prerequisites

_**Note:** The instructions on this page use the Kubernetes `kubectl` command (for portability reasons across Kubernetes environments). For environments, the `kubectl` command is an alias that points to the OpenShift client program `oc`._

### 1. Provision a Kubernetes cluster

Create a Kubernetes cluster and connect to the cluster. 
In our verification tests, we regularly verify the samples workloads outlined on this page using the following minimal configuration:
* 5 nodes, each with with 2 CPUs and 16 GB of RAM
* 5 GB disk for Admin pods
* 20 GB disk for Storage Manager(SM) pods

Please use this as a guideline for a minimal configuration when you create your cluster. To run larger SQL workloads using the included YCSB sample application, adjust node CPU and Memory upwards as required. To determine resources used, monitor your NuoDB database process resource consumption using the NuoDB Insights visual montioring tool. 

### 2. Create environment variables

```
export OPERATOR_NAMESPACE=nuodb
export NUODB_OPERATOR_VERSION=2.0.3           --confirm you set the correction NuoDB Operator version here.
```
### 3. Clone a copy of the NuoDB Operator from Github
In your home or working directory, run:

`git clone https://github.com/nuodb/nuodb-operator`

### 4. Create the "nuodb" project/namespace (if not already created)

`kubectl create namespace $OPERATOR_NAMESPACE`

### 5. Optionally Use Cluster Node Local Storage

NuoDB supports cloud platform storage (e.g. AWS EBS, GCP PD, Azure Disk, etc), 3rd-party CSI storage (e.g. Portworx, OpenEBS, Linbit, etc.), and the use of local storage via Hostpath. If planning to use any of these storage types, configure them prior to deploying your database. 

#### To Setup local storage (using HOSTPATH): 
Configure the local storage permissions on each cluster node to enable hosting storage for either the NuoDB Admin or the Storage Manager (SM) pods.
**Note:** When using the local disk storage option only 1 Admin pod is supported.

```
sudo mkdir -p /mnt/local-storage/disk0
sudo chmod -R 777 /mnt/local-storage/
sudo chcon -R unconfined_u:object_r:svirt_sandbox_file_t:s0 /mnt/local-storage
sudo chown -R root:root /mnt/local-storage
```
Create the Kubernetes storage class "local-disk" and persistent volume

 `kubectl create -f nuodb-operator/build/etc/charts/nuodb-helm/local-disk-class.yaml`

### 6. Cluster Node Labeling
Label the cluster nodes you want to use to run NuoDB pods.

 `kubectl  label node <node name> nuodb.com/zone=nuodb`

_**Note:** The label value, in this example "nuodb", can be any value._

Next, label one of these nodes as your storage node that will host the NuoDB Storage Manager (SM) pod. If using Local storage, ensure there is sufficient disk space on this node. To create this label run:

`kubectl  label node <yourStorageNodeDNSName> nuodb.com/node-type=storage`

Once your cluster nodes are labeled for NuoDB use, run the following `kubectl get nodes` command to confirm nodes are labeled properly. The display output should look similar to the below
```
kubectl get nodes -l nuodb.com/zone -L nuodb.com/zone,nuodb.com/node-type
NAME                           STATUS   ROLES    AGE   VERSION             ZONE    NODE-TYPE
ip-10-0-141-113.ec2.internal   Ready    worker   15d   v1.13.4+cb455d664   nuodb   storage
ip-10-0-152-147.ec2.internal   Ready    worker   15d   v1.13.4+cb455d664   nuodb   
ip-10-0-162-73.ec2.internal    Ready    worker   15d   v1.13.4+cb455d664   nuodb   
ip-10-0-184-233.ec2.internal   Ready    worker   15d   v1.13.4+cb455d664   nuodb   
ip-10-0-206-8.ec2.internal     Ready    worker   15d   v1.13.4+cb455d664   nuodb 
```

### 7. Apply a NuoDB license file

Each time a NuoDB Admin pod starts it will load a Kubernetes configmap that contains the current NuoDB license level information and places its contents in the /etc/nuodb/nuodb.lic file. When a request is made to either start a NuoDB Transaction Engine (TE) or Storage Manager (SM) process, the NuoDB Admin will check the license file contents to ensure the process request is authorized.

To apply a NuoDB Communiity Edition (CE) license file, run

`kubectl create configmap nuodb-lic-configmap -n $OPERATOR_NAMESPACE --from-literal=nuodb.lic=""`

To apply a NuoDB Enterprise Edition (EE) license file to a running NuoDB database system that is using a CE license, 

obtain your EE license file from your NuoDB Sales or Support representative and copy the file to `nuodb.lic`, then run

```
kubectl delete configmap nuodb-lic-configmap -n $OPERATOR_NAMESPACE
kubectl create configmap nuodb-lic-configmap -n $OPERATOR_NAMESPACE --from-file=nuodb.lic
```
Then, delete a NuoDB Admin pod, and once the Admin pod has been restarted, connect to the new pod and run,

`nuocmd set license --license-file /etc/nuodb/nuodb.lic`

This command will propagate the new NuoDB EE license throughout the Admin tier (remaining pods).  

**Note:** The filename specified in the above commands must be nuodb.lic

To check the effective NuoDB license and confirm license level, run

`nuocmd --show-json get effective-license`


### 8. If using the Red Hat OpenShift 

#### To permit the pulling of the NuoDB database and operator container images, create the Kubernetes image pull secret

This secret will be used to pull the NuoDB Operator and NuoDB container images from the  Red Hat Container
Catalog (RHCC). Enter your Red Hat login credentials for the --docker-username and --docker-password values.

```
kubectl  create secret docker-registry pull-secret \
   --docker-username='yourUserName' --docker-password='yourPassword' \
   --docker-email='yourEmailAddr'  --docker-server='registry.connect.redhat.com'
 ```
**Note:** If using Quay.io (or other supported public repo) to pull the NuoDB container images, a Kubernetes secret is not required because the NuoDB repository is public. For example, to pull the image from quay.io, run at the command prompt, docker pull quay.io/nuodb/nuodb-operator.

#### Disable Linux Transparent Huge Pages (THP). Run the following required command to create a security context constraint which will allow the Operator to disable THP during Operator deployment.
```
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/thp-scc.yaml
```
#### Run the following oc admin policy commands,
```
oc adm policy add-scc-to-user privileged system:serviceaccount:$OPERATOR_NAMESPACE:nuodb-operator
oc adm policy add-scc-to-user privileged system:serviceaccount:elastic-system:elastic-operator
oc adm policy add-scc-to-user privileged system:serviceaccount:$OPERATOR_NAMESPACE:insights-server-release-logstash
```

## Configure NuoDB Insights Visual Monitor

![](https://www.nuodb.com/sites/default/files/nuodb-insights.png)

Optionally deploy the NuoDB Insights visual monitoring tool **(recommended)**. NuoDB Insights is a powerful database monitoring tool that can greatly aid in visualizing database workload and resource consumption. For more information about the benefits of Insights, and data privacy when using `HOSTED`, please refer to the [NuoDB Insights](https://www.nuodb.com/product/insights) Webpage.

Before deploying  NuoDB, to enable NuoDB Insights you will need to choose one of the available deployment methods: 
1. LOCAL: Deploy Insights locally on your Kubernetes cluster. With this option, all performance data is privately stored and managed locally on your cluster by starting local elasticsearch, logstash, kibana, and grafana components that are utilized by the Insights on-cluster monitoring solution. To enable this option: 
   * Apply the `nuodbinsightsserver_crd.yaml` during Operator deployment and `nuodbinsightsserver_cr.yaml` during database deployment.
   
2. HOSTED in AWS: This option streams your NuoDB Insights performance data to the NuoDB hosted Insights data portal on the Amazon AWS public cloud. To access your secure performance data using this option, you will use a private Subscriber ID that will be provided to you once the Insights collection agent starts. 
**Note:** Before selecting this option, you must agree to our NuoDB Terms of Use and Data Privacy Policy located here:
[Terms of Use](https://www.nuodb.com/terms-use) and [Privacy Policy](https://www.nuodb.com/privacy-policy). If you agree to these terms, following the below instructions to enable HOSTED NuoDB Insights. This option can also be enabled at a later time if you choose.
 To enable this option: 
   * Set `insightsEnabled` paramater to `true` in your `nuodb-cr.yaml` file. By setting this option, this is your acknowledgement and acceptance of the NuoDB terms of use and data Privacy policy. 

## Install the NuoDB Operator

To install the NuoDB Operator into your Kubernetes cluster, follow the steps indicated for the appropriate Kubernetes Distribution you are using.

### Red Hat OpenShift v4.x 

In OpenShift 4.x, the NuoDB Operator is available to install directly from the OpenShift OperatorHub, an integrated service catalog, accessible from within the OpenShift 4 Web UI which creates a seamless - single click experience - that allows users to install the NuoDB Operator from catalog-to-cluster in seconds.

Prerequisite: 
Run the following yaml in your OpenShift cluster to authorize the NuoDB Operator service account before installing the NuoDB Operator.
```
kubectl create -f nuodb-operator/deploy/cluster-op-admin.yaml
```
Steps:
1. Select `Projects` from the OpenShift 4 left toolbar and click the `NuoDB` project to make
   it your current project.
2. Select the `OperatorHub` under the `Catalog` section in the OCP 4 left toolbar.
3. Select the `Database` filter and scroll down to the NuoDB Application tile and click the tile.
4. In the right-hand corner of the NuoDB Operator page, click the `Install` button.
5. On the "Create Operator Subscription" page, select the radio group option "A specific namespace on the cluster"
   and enter the project/namespace in the pull-down field that you would like to install the NuoDB Operator,
   then select `Subscribe` to subscribe to the NuoDB Operator.
6. In less than a minute, on the page that displays should indicate the NuoDB Operator has been
   installed, see "1 installed" message.
7. To verify the NuoDB Operator installed correctly, select `Installed Operators` from the left
   toolbar. The STATUS column should show "Install Succeeded".
8. Select `Status` under the `Projects` on the left toolbar to view your running Operator.

The following video provides a full walk-thru of how to deploy the NuoDB Operator and database in OpenShift 4.x. 

&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;[NuoDB in OpenShift  v4.x video](https://youtu.be/KYx_B_ykbtU)

**Note:** The same benefit areas can also be demonstrated in any NuoDB supported Kubernetes managed environment.

### Google Cloud Platform (GCP) - GKE Kubernetes

1. Using the GCP Marketplace, locate the NuoDB Operator. Click the `Configure` botton and follow the on screen instructions to deploy the NuoDB Operator. During this step: 
2. Select a GCP Project. 
3. Either create a GKE cluster or choose an existing one from the list
4. Create a namespace called `nuodb` in which to install the NuoDB Operator
5. Take defaults for `App instance name` and `Cluster Admin Service Account` 
6. Click the `Deploy` button.
Your NuoDB Operator will deploy in several minutes. You can use the GCP Kubernetes Engine Web UI "Workloads" selection to monitor progress.

### Red Hat OpenShift v3.11 --or-- Open source, RKE, Anthos GKE, and EKS Kubernetes

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

#### NuoDB Operator Linux CLI Install Script
```
# Set the environment context to the namespace you will deploy the NuoDB Operator
kubectl config set-context --current --namespace=$OPERATOR_NAMESPACE

kubectl create -f nuodb-operator/deploy/catalogSource.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/operatorGroup.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/cluster_role.yaml
kubectl create -f nuodb-operator/deploy/cluster_role_binding.yaml
kubectl create -f nuodb-operator/deploy/cluster-op-admin.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/role.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/role_binding.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/service_account.yaml

## add NuoDB, Insights, and ycsb sample SQL app CRDs
kubectl create -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodb_crd.yaml
kubectl create -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbinsightsserver_crd.yaml
kubectl create -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbycsbwl_crd.yaml

# create a local copy of the nuodb-csv.yaml file
sed "s/placeholder/$OPERATOR_NAMESPACE/" nuodb-operator/deploy/olm-catalog/nuodb-operator/$NUODB_OPERATOR_VERSION/nuodb-operator.v$NUODB_OPERATOR_VERSION.clusterserviceversion.yaml > nuodb-csv.yaml

# To replace quay.io as the default location to pull the NuoDB Operator image, follow these examples:

   # Create a new nuodb-csv.yaml flie that pulls from the Red Hat Container Catalog, run
   #   sed "s/quay.io/registry.connect.redhat.com/" nuodb-csv.yaml > nuodb-csv-rhcc.yaml

   # Create a new nuodb-csv.yaml file that pulls from the Google Marketplace, run
   #   sed "s/quay.io/marketplace.gcr.io/"          nuodb-csv.yaml > nuodb-csv-gcp.yaml

   # To pull from the AWS Marketplace, make a copy of nuodb-csv.yaml and name it nuodb-csv-aws.yaml.
   # Replace in the nuodb-csv-aws.yaml file the two image references with the following image pull value:
   # 117940112483.dkr.ecr.us-east-1.amazonaws.com/d893f8e5-fe12-4e43-b792-8cb98ffc11c0/cg-756769224/quay.io/nuodb/nuodb-operator:2.0.3-1-latest

# If appliable, copy your customized nuodb-csv-xxx.yaml file to nuodb-csv.yaml and run,
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-csv.yaml

# Check deployment rollout status every 5 seconds (max 10 minutes) until complete.
ATTEMPTS=0
ROLLOUT_STATUS_CMD="kubectl rollout status deployment/nuodb-operator -n $OPERATOR_NAMESPACE"
until $ROLLOUT_STATUS_CMD || [ $ATTEMPTS -eq 60 ]; do
  ATTEMPTS=$((attempts + 1))
  echo ""
  kubectl get pods -n $OPERATOR_NAMESPACE
  sleep 5
done
```

## Deploy the NuoDB Database

### Sample Database deployment files

The nuodb-operator/deploy/crds directory includes sample Custom Resources yaml files that can be used to deploy a NuoDB database with on-cluster (locally deployed) NuoDB Insights, and a ycsb sample workload. 

`nuodb_v2alpha1_nuodb_cr.yaml`
`nuodb_v2alpha1_nuodbinsightsserver_cr.yaml`
`nuodb_v2alpha1_nuodbycsbwl_cr.yaml`

Optionally, you can add any of the below parameters values to your own customer resource files to customize your deployment. Each parameter is described in the &nbsp;[Optional Database Parameters](#Optional-Database-Parameters) section. Sample deployment files are provided below. 

#### nuodb_v2alpha1_nuodb_cr.yaml

This sample file starts a database named *test*,  uses persistent storage, disables *hosted* Insights monitoring, starts three NuoDB Admin pods, and includes various others configurations like controlling the number desired pods, CPU, and memory used per NuoDB process type.
```
spec:
  replicaCount: 1
  storageMode: persistent
  insightsEnabled: false
  adminCount: 3
  adminStorageSize: 2G
  adminStorageClass: <ENTER VALUE>
  dbName: test
  dbUser: dba
  dbPassword: secret
  smCount: 1
  smMemory: 4Gi
  smCpu: "2"
  smStorageSize: 20G
  smStorageClass: <ENTER VALUE>
  engineOptions: ""
  teCount: 1
  teMemory: 4Gi
  teCpu: "2"
  container: nuodb/nuodb-ce:latest
```

We recommend replacing the database password `dbPassword` value 'secret' with one of your choice. Also, it's common to configure the image pull source locations by replacing the default values for the `ycsbContainer` and `container` parameters with values that match your deployment type.

#### nuodb_v2alpha1_nuodbinsightsserver_cr.yaml

```
spec:
  elasticVersion: 7.3.0
  elasticNodeCount: 1
  kibanaVersion: 7.3.0
  kibanaNodeCount: 1
  storageClass: <ENTER VALUE>
```
**Note:**  For parameters `adminStorageClass`, `smStorageClass`, and `storageClass` enter the Kubernetes storage class value you wish to use. For example, 

| Public Cloud | Kubernetes Storage Class    |
|--------------|-----------------------------|
| AWS          | gp2                         |
| GCP          | standard                    |
| AZURE        | standard_lrs or premium_lrs |

| User Config'd  | Kubernetes Storage Class   |
|----------------|----------------------------|
| Local Hostpath | local-disk                 |
| 3rd Party      | vendor specific value      |

#### nuodb_v2alpha1_nuodbycsbwl_cr.yaml
```
spec:
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

### Sample NuoDB database deployment script

This sample deploys a NuoDB database using "on-cluster" NuoDB Insight visual monitoring and start a sample SQL application
```
# Set the environment context to the namespace you will deploy the NuoDB Operator
kubectl config set-context --current --namespace=$OPERATOR_NAMESPACE`

# To deploy the NuoDB database into your Kubernetes cluster, first make a local copy of the NuoDB cr yaml files
cp nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodb_cr.yaml                 nuodb-cr.yaml
cp nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbinsightsserver_cr.yaml   nuodb-insights-cr.yaml
cp nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbycsbwl_cr.yaml           nuodb-ycsbwl_cr.yaml

# add cluster-admin permissions to the nuodb-operator service account                               
kubectl create -f nuodb-operator/deploy/cluster-op-admin.yaml

# Modify / customize your NuoDB cr yaml files and run, (see samples below in next section)
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-cr.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-insights-cr.yaml
kubectl create -n $OPERATOR_NAMESPACE -f nuodb-ycsbwl-cr.yaml

#Wait for nuodb to be logstash instance to be ready
# Check deployment rollout status every 10 seconds (max 10 minutes) until complete.
ATTEMPTS=0
ROLLOUT_STATUS_CMD="kubectl rollout status sts/insights-server-release-logstash -n $OPERATOR_NAMESPACE"
until $ROLLOUT_STATUS_CMD || [ $ATTEMPTS -eq 60 ]; do
  ATTEMPTS=$((attempts + 1))
  echo ""
  kubectl get pods -n $OPERATOR_NAMESPACE
  sleep 5
done

# create the Insights client
kubectl create -f nuodb-operator/build/etc/insights-server/insights-client.yaml
 
echo "Obtain your NuoDB Insights Dashboard URL:"
echo "For Red Hat OpenShift, go to URL:"
echo "   https://$(kubectl get route grafana-route --output=jsonpath={.spec.host})/d/000000002/system-overview?orgId=1&refresh=10s"

echo "For Google GKE, go to URL:"
echo "   http://$(kubectl get ingress grafana-ingress --output=jsonpath={.status.loadBalancer.ingress[0].ip})/d/000000002/system-overview?orgId=1&refresh=10s"

echo "For EKS or open source K8S,"
echo "Run the following command in a terminal window suitable for logging output commands:"
echo "   $ kubectl port-forward service/grafana-service 3000 &"
echo "Go to URL:"
echo "   localhost:3000/d/000000002/system-overview?orgId=1&refresh=10s"   
 ```

#### If deploying on-cluster NuoDB Insights
To obtain your on-cluster NuoDB Insights URL,

For Red Hat OpenShift, go to URL:
```
echo "   https://$(kubectl get route grafana-route --output=jsonpath={.spec.host})/d/000000002/system-overview?orgId=1&refresh=10s"
```
For Google GKE, go to URL:
```
echo "   http://$(kubectl get ingress grafana-ingress --output=jsonpath={.status.loadBalancer.ingress[0].ip})/d/000000002/system-overview?orgId=1&refresh=10s"
```
For EKS or open source K8S, run the following command in a terminal window suitable for logging output commands:
```
$ kubectl port-forward service/grafana-service 3000 &"
```
Go to URL:
```
localhost:3000/d/000000002/system-overview?orgId=1&refresh=10s
```
**Note:** It may take several minutes for the NuoDB Insights URL to become available. 


#### If deploying hosted NuoDB Insights
Optionally, you can choose to send your performance data to the NuoDB publicly hosted Insights portal. Your performance data remains private and is only accessible by using your private Subscriber ID. With this option, you can find your NuoDB Insights SubcriberID by locating the "nuodb-insights" pod in your Kubernetes dashboard, go to the Logs tab, and find the line that indicates your Subscriber ID. 
```
Insights Subscriber ID: yourSubID#
```
**Note:** When using the open source Kubernetes dashboard:** A current Kubernetes dashboard Web UI issue doesn't allow users to retrieve their Insights Subscription ID using the dashboard to inspect the nuodb-insights log file. Instead run,
```
kubectl logs nuodb-insights -n nuodb -c insights
```
To connect to NuoDB Insights, open a Web browser using the following URL

https://insights.nuodb.com/yourSubID#

To check the status of hosted NuoDB Insights visual monitoring tool, run

`oc exec -it nuodb-insights -c insights -- nuoca check insights`

## Connect to your Database using nuosql

Once your database is running, you can connect and run SQL using the NuoDB nuosql command tool. See sample below:

```
kubectl exec -it <te-pod-name> bash
nuosql <db-name> --user dba --password secret
SQL>
```
For more information on how to run SQL, see [Using NuoDB SQL Command Line](http://doc.nuodb.com/Latest/Default.htm#Using-NuoDB-SQL-Command-Line.htm) and [NuoDB SQL Development](http://doc.nuodb.com/Latest/Default.htm#SQL-Development.htm) online documenation.

## Launch a Sample SQL Workload

The NuoDB Operator includes a sample SQL application that will allow you to get started quickly running SQL statements against your NuoDB database. The sample workload uses YCSB (the Yahoo Cloud Servicing Benchmark). The cr.yaml includes YCSB parameters that will allow you to configure the SQL workload to your preferences.

To start a SQL Workload (if your `nuodb-ycsbwl-cr.yaml` wasn't configured to start one by default) locate the ycsb Replication Controller in your Kubernetes dashboard and scale it to your desired number of pods to create your desired SQL application workload. Once the YCSB application is running the resulting SQL workload will be viewable from the NuoDB Insights visual monitoring WebUI.

## NuoDB Features and Benefits Evaluation Steps

Once your NuoDB database and Insights visual monitor are running, here are a few steps to try out to quickly realize the benefits of running a NuoDB SQL database

* Demonstrate Transactional Scale-out

    To easily scale out your NuoDB Transaction Engines (TEs) to meet an increased SQL application workload, from the CLI, edit the teCount value by running,

    `kubectl edit nuodbs.nuodb.com`
    
    Using NuoDB Insights, monitor the SQL application during the time of increased application workload and the scale-out of TEs and observe the increase in transactional throughput, Transactions Per Second (TPS) 
    
* Demonstrate Continuous Availability

    Deploy your NuoDB system with three NuoDB Admins and three Transaction Engines (TEs). Using either your Kubernetes dashboard or the `kubectl delete pod` CLI command, forcibly delete either a NuoDB Admin or a TE pod. **Note:** when using the Community Edition, only 1 Storage Manager (SM) is availabe. 
    
    Using NuoDB Insights, monitor the SQL application during the time of the forced pod deletes and observe that SQL application availability continues unimpacted while the Kubernetes system and NuoDB recovery from failure events in seconds.

## Delete the NuoDB database
```
kubectl delete -n $OPERATOR_NAMESPACE configmap nuodb-lic-configmap

kubectl delete pod/insights-client
kubectl delete -f nuodb-cr.yaml
kubectl delete -f nuodb-insights-cr.yaml
kubectl delete -f nuodb-ycsbwl-cr.yaml

# Delete the NuoDB persistent storage volumes claims
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
kubectl delete pod/insights-client
kubectl delete nuodbinsightsservers/insightsserver
kubectl delete nuodbs/nuodb-db
kubectl delete nuodbycsbwls/nuodbycsbwl
kubectl delete pvc --all 
kubectl delete pv --all

kubectl delete -f nuodb-operator/deploy/catalogSource.yaml
kubectl delete -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/operatorGroup.yaml
kubectl delete -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/cluster_role.yaml
kubectl delete -f nuodb-operator/deploy/cluster_role_binding.yaml
kubectl delete -f nuodb-operator/deploy/cluster-op-admin.yaml
kubectl delete -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/role.yaml
kubectl delete -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/role_binding.yaml
kubectl delete -n $OPERATOR_NAMESPACE -f nuodb-operator/deploy/service_account.yaml

kubectl delete -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodb_crd.yaml
kubectl delete -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbinsightsserver_crd.yaml
kubectl delete -f nuodb-operator/deploy/crds/nuodb_v2alpha1_nuodbycsbwl_crd.yaml

kubectl delete -f nuodb-csv.yaml

kubectl delete clusterrolebinding nuodb-op-admin

# For OPENSHIFT only, delete the thp security context constraint
kubectl delete scc thp-scc

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

&ensp; `smMemory: 2Gi`


**smCpu** - SM CPU cores to request

&ensp; `smCpu: "1"`


**smStorageSize** - Storage manager (SM) volume size (GB)

&ensp; `smStorageSize: 20G`


**smStorageClass** - SM persistent storage class name

&ensp; `smStorageClass: local-disk`


**engineOptions** - Additional "nuodb" engine options Format: …

&ensp; `engineOptions: ""`


**teCount** - Number of transaction engines (TE) nodes. Limit is 3 in CE version of NuoDB

&ensp; `teCount: 1`


**teMemory** - TE memory (in GB)

&ensp; `teMemory: 2Gi`


**teCpu** - TE CPU cores to request

&ensp; `teCpu: "1"`

**apiServer** - Load balancer service URL. hostname:port (or LB address) for nuocmd and nuodocker process to connect to.

&ensp; `apiServer: https://domain:8888`


**container** - NuoDB fully qualified image name (FQIN) for the Docker image to use

Below are examples that pull the NuoDB container image from Red Hat (RHCC), Google Cloud Platform Marketplace, AWS Marketplace, and DockerHub.

```
container: registry.connect.redhat.com/nuodb/nuodb-ce:latest
container: marketplace.gcr.io/nuodb/nuodb:latest
container: 117940112483.dkr.ecr.us-east-1.amazonaws.com/d893f8e5-fe12-4e43-b792-8cb98ffc11c0/cg-756769224/docker.io/nuodb/nuodb-ce:2.0.3-1-latest
container: nuodb/nuodb-ce:latest
```


## Optional YCSB Workload Parameters

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


**ycsbContainer** - YCSB fully qualified image name (FQIN) for the ycsb docker image to use. See examples below pulling the image from dockerhub and the AWS Marketplace.

```
ycsbContainer: nuodb/ycsb:latest
ycsbContainer: 117940112483.dkr.ecr.us-east-1.amazonaws.com/d893f8e5-fe12-4e43-b792-8cb98ffc11c0/cg-756769224/docker.io/nuodb/ycsb:2.0.3-1-latest
```


## Optional NuoDB Insights-Server Parameters

**elasticVersion** - Version of ElasticSearch

&ensp; `elasticVersion: 7.3.0`

**elasticNodeCount** - Number of nodes in the ElasticSearch Cluster

&ensp; `elasticNodeCount: 1`

**kibanaVersion** - Version of Kibana

&ensp; `kibanaVersion: 7.3.0`

**kibanaNodeCount** - Version of Kibana

&ensp; `kibanaNodeCount: 1`

**storageClass** - Kubernetes Persistent Storage Class

&ensp; `storageClass: ""`


## Building the NuoDB Operator

The NuoDB Operator is built using the K8s [Operator SDK](https://github.com/operator-framework/operator-sdk).  The CRDs (deploy/crds/*_crd.yaml, OpenAPI (zz_generated.openapi.go), and deep copy functions (zz_generated.deepcopy.go) are automatically generated using the Operator SDK commands:

```
operator-sdk generate k8s
operator-sdk generate openapi
```
