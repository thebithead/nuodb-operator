#! /bin/bash
#
# Setup OCP System E2E Test envionment.
#
#   source ocp_e2e_test_env.sh <full-path-to-openshift-install-test-directory>
#
echo "Setup OCP System E2E Test environment."
if [ ! -d "${1}" ]; then
    echo "Error: Unable to find '${1}'"
    exit 1
fi
export TEST_DIR=$1
echo "Test Directory: $TEST_DIR"
cd "${TEST_DIR}"
export PATH="${TEST_DIR}":"${PATH}"
export KUBECONFIG="${TEST_DIR}/auth/kubeconfig"
export KUBEADMIN_PASSWORD=$(cat "${TEST_DIR}/auth/kubeadmin-password")
oc login --username=kubeadmin --password=${KUBEADMIN_PASSWORD}
