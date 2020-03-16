#! /bin/bash
set -e
#
# Provision an OpenShift 4.x cluster using the openshift-install tool:
#
#    https://docs.openshift.com/container-platform/4.2/installing/installing_aws/installing-aws-account.html
#
# Users must provide their own install configuration with their own authorization
# credentials and SSH Key as a install-config.yaml passed in as the
# first argument to this scipt.  Run it like this:
#
#   openshift_provision.sh install-config.yaml
#
################################################################################
# Start of example install-config.yaml for provisioning OpenShift on AWS.
################################################################################
# 
# apiVersion: v1
# baseDomain: openshift.nuodb.io
# compute:
# - hyperthreading: Enabled
#   name: worker
#   platform:
#     aws:
#       type: m4.xlarge            <=== AWS Instance Size.
#   replicas: 4                    <=== Quantity of K8s Nodes.
# controlPlane:
#   hyperthreading: Enabled
#   name: master
#   platform: {}
#   replicas: 1
# metadata:
#   creationTimestamp: null
#   name: nuodb-op-test30          <=== Name of your choice.
# networking:
#   clusterNetwork:
#   - cidr: 10.128.0.0/14
#     hostPrefix: 23
#   machineCIDR: 10.0.0.0/16
#   networkType: OpenShiftSDN
#   serviceNetwork:
#   - 172.30.0.0/16
# platform:
#   aws:
#     region: ca-central-1
# publish: External
# pullSecret: <pull secret>
# sshKey: |
# ssh-rsa <key> <email>
#
################################################################################
# End of example install-config.yaml for provisioning OpenShift on AWS.
################################################################################
#
echo "Provision OpenShift on AWS"

if [ "x${1}" == "x" ]; then
    echo "  Usage:   openshift_provision <install-yaml-file>"
    echo "  Example: openshift_provision my-install-config.yaml"
    exit 1
fi

if [ ! -f "${1}" ]; then
    echo "  Error: Unable to find '${1}'"
    echo "  Usage: openshift_provision <install-yaml-file>"
    exit 1
fi

# Set operating system
if [ "$(uname)" == "Darwin" ]; then
    opsys=mac
else
    opsys=linux
fi

echo "Script running on $opsys"

ocp_version=4.2.12
echo "OCP Version: $ocp_version"
current_dir=$(pwd)
echo "Current Directory: $current_dir"

# Do this once, into the current directory

# Get client side tools: kubectl and oc
CLIENT_GZIP=openshift-client-${opsys}-${ocp_version}.tar.gz

if [ ! -f "${CLIENT_GZIP}" ]; then
    wget https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${ocp_version}/${CLIENT_GZIP}
fi
tar xvzf ${CLIENT_GZIP}

if [ -f "oc" ]; then
    echo "Client tools oc and kubectl available"
else
    echo "Unable to download client tools"
    exit 1
fi

# Get the OpenShift installer
INSTALLER_GZIP=openshift-install-${opsys}-${ocp_version}.tar.gz

if [ ! -f "${INSTALLER_GZIP}" ]; then
    wget https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${ocp_version}/${INSTALLER_GZIP}
fi
tar -xvzf ${INSTALLER_GZIP}

if [ -f "openshift-install" ]; then
    echo "OpenShift installer is available"
else
    echo "Unable to download OpenShift installer tools"
    exit 1
fi

# Create a unique working directory for all the artefacts created during installation
if [ "$(uname)" == "Darwin" ]; then
    timestamp=`date +"%Y-%m-%dT%H:%M:%SZ"|sed "s/:/-/g"`
else
    timestamp=`date --iso-8601=ns|sed "s/,/./g"|sed "s/:/-/g"`
fi

export TEST_DIR=${current_dir}/openshift-install.${timestamp}

echo "Working (installation) directory: $TEST_DIR"
mkdir "${TEST_DIR}"

# The install deletes the YAML file, so we  copy the original
cp "${1}" "${TEST_DIR}/install-config.yaml"

# Change into the working directory
cd "${TEST_DIR}"

# Do the installation - this uses the YAML file
date
echo "Creating the OpenShift Cluster"
../openshift-install create cluster
date

# Success - login to the cluster using oc
export KUBECONFIG="${TEST_DIR}/auth/kubeconfig"
export KUBEADMIN_PASSWORD=`cat "${TEST_DIR}/auth/kubeadmin-password"`

echo KUBECONFIG=$KUBECONFIG
echo KUBEADMIN_PASSWORD=$KUBEADMIN_PASSWORD
echo "Logging into your new cluster"
oc login --username=kubeadmin --password=${KUBEADMIN_PASSWORD}

#
# Required K8s Node labels.
kubectl get node -o custom-columns=NODE:.metadata.name|tail -n+2|xargs -I "kubectl label node {}  nuodb.com/zone=a"
kubectl get node -o custom-columns=NODE:.metadata.name|tail -n+2|xargs -I "kubectl label node {}  nuodb.com/node-type=storage"

echo "OpenShift installed. Look in openshift-install.${timestamp} for more details"
echo "   You will find the kubeadmin password and kubeconfig file in openshift-install.${timestamp}/auth"
echo "   The installation log is in openshift-install.${timestamp}/.openshift_install.log"

#
# To deprovision the entire cluster, simply execute:
#
# cd $TEST_DIR; ./openshift-install destroy cluster
