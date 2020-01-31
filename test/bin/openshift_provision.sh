#! /bin/bash
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
# pullSecret: '{"auths":{"cloud.openshift.com":{"auth":"obscurecredsssslbGffc2UtZGV2K3RvbWdsdsVzbnVvZGIxa2kxMWVjdjIxcTNsa3NjYXQ4aWxvdjR1MXg6Tk5XUVZLNEVKMlBXSENWMlJKVVRQV05OSjNONDZJTVkxUlhZTUlaVstFWVZWTEdMRldPWUlWQk0xRElTV0IzMg==","email":"tgates@nuodb.com"},"quay.io":{"auth":"obscurecredsssslsGVhc2UtZGV2K3RvbWdhdGVzbnVvZGIxa2kxMWVjdjIxcTNsa3NjYXQ4aWxvdjR1MXg6dk5XUVZLNEVKMlBXSENWMlJKVVRQd05OSjNONDZJTVkxUlhZTUlaVUtFWVZWTEdMRldPWUlWQk0xRElTV0IzMg==","email":"tgates@nuodb.com"},"registry.connect.redhat.com":{"auth":"obscure-credsssjLTFLSTExRWN2MjFxM2xrc0NBVdhpsG9dNHUxWDpleUpoYkdjaU9pSlNVelV4TWlKOS5leUp6ZFdJaU9pSmtPRGswTlRnMVlXRmlNemMwWkRRME9HRTNaR1V6WkdJNU5tUXdZafxoWWlKOS5tZ01rVzg2azFNSDVFb1BFVmFRTi1YVEZvRXlSdURNMXY4aElXUk9wZslwfVRfekllM2ZZTXpyY3NoMllOem1VMC1qakZsUm5ib2hLN2FWZ3hNd1ZWM01JM1BQeXR4amloX1lvbUFkRk9GNkVKSkFBOUpnWsBzTDdGf2ZJMXBQNUU4dWxFYVZsMS1nR3pPdXlKY0hvVHItcTJfVG5UR2J2dEx3aFJJdlNvT01XRzFkcDZZRUlfMEdjNFpDT0wwQsd4WnJoa29EU1JTbTdmQ1FyNHNMaExScVUxWndPeEpjT1hyaUlGT0ozNkg1TXhGWnpUR0Y4UDJMTDJxS3llQnN5NnhZOXBBSU50UXJPRTZMV3RUVmNLR2FEb2c0RFItUUVnYkh6VFhMRVlHMEp4M2Nta2pObGlTYVVyLVNfSk94dldTS01WaEdTRkx6SnU3UmVGdHFYOXB1NmpFZUZqRVBWYkREdk5leFJ3X3JiUGdWMmpvN0xBM0J3TWtLR2FuRTNGYXJHUXM4X2tyRzhEZnhNdnFuNWJfUXRVUTJvcnZBOFd3bE5FOFhEV1J5MlU2bjNmNjZkVnhJZzFINEV4YzJBTENCR0J5cjI4RnBjbFFLUWxRRWEwOE5jWHVXWWJtMGlYRDdNbThKYmw4WlFNbF94aS1GeXZqc25GQTFnaENSRk5ITWRXR2l5VERVaFhMeU9JOVdWTEp2LS1mWXBHeXM3emxNdWVOeHI1dGNEb09qR040blNQS1U3M3F6bHBBVkFBek5uQ0RadXdYN1pVaVRwWW9QeUdVSE5zUXNCajk3SDNwUXdwbzBjWkVYaWpIelVnSkNzMVY2WUktMzVITHJEWXNXV2hJbnRRY0R5UDBIU1dpWGJHVXE3SEJOY3F4MTg5M0VELU9vTXpHSQ==","email":"tgates@nuodb.com"},"registry.redhat.io":{"auth":"obscurecredssssjsTgLSTExRWN2MjFxM2xrc0NBVDhpbG9WNHUxWDpleUpoYkdjaU9pSlNVelV4TWlKOS5leUp6ZFdJaU9pSmtPRGswTlRnMVlXRmlNemMwWkRRME9HRTNaR1V6WkdJNU5tUXdZamxoWWlKOS5tZ01rVzg2azFNSDVFb1BFVmFRTi1YVEZvRXlSdURNMXY4aElXUk9wZslwQVRlekllM2ZZTXpyY3NoMllOem1VMC1qakZsUm5ib2hLN2FWZ3hNd1ZWM01JM1BQeXR4amloX1lvbUFkRk9GNkVKSkFBOUjnfXBzTDdGN2ZJMXBQNUU4dWxFYVZsMS1nR3pPYXlKY0hvVHItcTJfVG5UR2J2dEx3aFJJdlNvT01XRzFkcDZZRUlfMEdjNFpkT0wwQzd4WnJoa29EU1JTbTdmQ1FyNHNMaExScVUxWndPeEpjT1hyaUlGT0ozNkg1TXhGWnpUR0Y4UDJMTDJxS3llQnN5NnhZOkBBSU50UXJPRTZMV3RUVmNLR2FEb2c0RFItUUVnYkh6VFhMRVlHMEp4M2Nta2pObGlTYVVyLVNfSk94dldTS01WaEdTRkx6SnU3UmVGdHFYOXB1NmpFZUZqRVBWYkREdk5leFJ3X3JiUGZWMmpvN0xBM0J3TWtLR2FuRTNGYXJHUXM4X2tyRzhEZnhNdnFuNWJfUXRVUTJvcnZBOgd3bE5FOFhEV1J5MlU2bjNmNjZkVnhJZzFINEV4YzJBTENCR0J5cjI4RnBjbFFLUWxRRWEwOE5jWHVXWWJtMGlYRDdNbThKYmw4WlFNbF94aS1GeXZqc25GQTFnaENSRk5ITWRXR2l5VERVaFhMeU9JOVdWTEp2LS1mWXBHeXM3emxNdWVOeHI1dGNEb09qR040blNQd1U3M3F6bHBBVkFBek5uQ0RadXdYN1pVaVRwWW9QeUdVSE5zUXNCajk3SDNwUXdwbzBjWkVYaWpIelVnSkNzMVY2WUktMzVITHJEWXNXV2hJbnRRY0R5UDBIU1dpWGJHVXE3SEJOY3s4MTg5M0VELU9vTXpHSQ==","email":"tgates@nuodb.com"}}}'
# sshKey: |
#   ssh-rsa obscurecredssssAAAfDAffffffffffffffffHHxpqaG1CRSuTxH8+uk/88ZLZGTMtriXz9cz2Prji4d3lpU3fd38OeaT22lRAddaFcBtW9r6X78i3qRSt8PXabqC7X1MqbPqToeeONDnWvsoVXvyQddddddddddddddddddddddlc9xYO3oWv1j+kcmo9augsUYsqMoXYPOcyj/F6pN2RSfo4YkbMhp1d/352ozAdvssssssssssssssqmgKk2eMxfIvg/683hw3dZfgfffffffffffffffff5NM7z+EPf6TKSUiJggggggggggggggggggggggggggggggSgmxw2sjgwTUsyTYlUTwNxDSV6iq5NgO6UoL tom@thebithead.com
#
################################################################################
# End of example install-config.yaml for provisioning OpenShift on AWS.
################################################################################
#
echo "Provision OpenShift"
if [ ! -f "${1}" ]; then
    echo "Error: Unable to find '${1}'"
    exit 1
fi
ocp_version=4.2.12
echo "OCP Version: $ocp_version"
current_dir=$(pwd)
echo "Current Directory: $current_dir"
timestamp=`date --iso-8601=ns|sed "s/,/./g"|sed "s/:/-/g"`
echo "Timestamp: $timestamp"
export TEST_DIR=${current_dir}/openshift-install.${timestamp}
echo "Test Directory: $TEST_DIR"
mkdir "${TEST_DIR}"
cp "${1}" "${TEST_DIR}/install-config.yaml"
cd "${TEST_DIR}"
wget https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${ocp_version}/openshift-client-linux-${ocp_version}.tar.gz
tar xvzf openshift-client-linux-${ocp_version}.tar.gz 
wget https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${ocp_version}/openshift-install-linux-${ocp_version}.tar.gz
tar -xvzf openshift-install-linux-${ocp_version}.tar.gz
date
export PATH="${TEST_DIR}":"${PATH}"
echo "Creating the OpenShift Cluster"
openshift-install create cluster
date
export KUBECONFIG="${TEST_DIR}/auth/kubeconfig"
export KUBEADMIN_PASSWORD=$(cat "${TEST_DIR}/auth/kubeadmin-password")
oc login --username=kubeadmin --password=${KUBEADMIN_PASSWORD}
#
# Required K8s Node labels.
kubectl get node -o custom-columns=NODE:.metadata.name|tail -n+2|xargs -i -t kubectl label node {}  nuodb.com/zone=a
kubectl get node -o custom-columns=NODE:.metadata.name|tail -n+2|xargs -i -t kubectl label node {}  nuodb.com/node-type=storage 

#
# To deprovision the entire cluster, simply execute:
#
#  cd $TEST_DIR; ./openshift-install destroy cluster
