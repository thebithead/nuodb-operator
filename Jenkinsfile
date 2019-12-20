  pipeline {
    agent {
      docker {
          image 'quay.io/nuodb/nuodb-operator-dev:golang-test'
          args '-v /var/run/docker.sock:/var/run/docker.sock -u 0:0 '
      }
    }

    
    parameters{
      string(defaultValue: 'quay.io/nuodb/nuodb-operator-dev', description: 'The image tag to be build and pushed', name: 'NUODB_OP_IMAGE', trim: false)
      choice(choices: ['OnPrem', 'AWS','EKS', 'GKE', 'AKS', 'OCP 4.x'], description: 'Specify A Environment to use for tests', name: 'CLUSTER_ENV')
      string(defaultValue: 'nuodb', description: 'Namespace for e2e tests', name: 'OPERATOR_NAMESPACE', trim: true)
      string(defaultValue: 'kops-test.openshift.nuodb.io', description: 'The Name of the cluster (e.g. kops-test.openshift.nuodb.io)', name: 'CLUSTER_NAME', trim: false)
      string(defaultValue: 's3://kops-state-store-jenkins', description: 'Kops state store bucket in us-east-1', name: 'KOPS_STATE_STORE', trim: true)
      string(defaultValue: 'Z24GEV7XM0UGHI', description: 'Private DNS zone id', name: 'DNS_ZONE_ID', trim: true)
    }


    stages {

      stage('Setup Environment for pipeline'){
        steps{
                sh '''
                go version
                operator-sdk version
                docker version
                '''
          }
        }

      stage('Checkout') {
        steps {
          checkout scm

          dir('kops-ansible') {
            git(
               url: 'git@github.com:nuodb/aws-kops.git',
               credentialsId: 'ashukla-git',
               branch: "master" 
            )
          }
        }
      }


      stage('Unit Tests') {
        steps {
            sh '''
            echo "Unit Testing"
            go test -v ./pkg/... -coverprofile=coverage.txt
            '''
        }
      }

      stage('Build operator and Image') {
        steps {
            sh '''
            env
            echo "Building operator image and pushing"
            PROJECT_NAME="nuodb-operator"
            CGO_ENABLED=0 
            GOOS=linux 
            GOARCH=amd64 

            go build \
              -o build/_output/bin/${PROJECT_NAME} cmd/manager/main.go

            docker build . -f build/Dockerfile -t $NUODB_OP_IMAGE:${GIT_COMMIT}
            '''
        }
      }

      stage('Push Image') {
        steps {
          script {
            withDockerRegistry([ credentialsId: "Quay-Robot", url: "https://quay.io/api/v1/" ]) {
             sh 'docker push $NUODB_OP_IMAGE:${GIT_COMMIT}'
            }
          }
        }
      }


      stage('E2E With OnPrem Environment') {
        when { expression { params.CLUSTER_ENV == 'OnPrem' }}
          steps {
            withKubeConfig([credentialsId: 'kubeconfig-onprem', serverUrl: 'https://10.3.100.81:6443']) {
              sh '''
              kubectl apply namespace $OPERATOR_NAMESPACE || true
              kubectl create secret docker-registry regcred --namespace=$OPERATOR_NAMESPACE --docker-server=quay.io --docker-username="nuodb+nuodbdev" --docker-password="RLT4418GQN01MVEUW9Q4I7P7ZZTQ1I7O9JZYNO3T8I7SX9WK0G4VK64MEAIKG3S5" --docker-email="" || true

              operator-sdk test local ./test/e2e --namespace $OPERATOR_NAMESPACE  --go-test-flags "-timeout 1200s" --verbose --image $NUODB_OP_IMAGE:${GIT_COMMIT}

              kubectl get pods -n $OPERATOR_NAMESPACE 

              ''' 
            }
          }
      }

      stage('E2E With AWS Environment') {
        when { expression { params.CLUSTER_ENV == 'AWS' }}
          steps {
            script {
            def built = build job: 'aws-kops', propagate: true, wait: true,  parameters: [[$class: 'StringParameterValue', name: 'CLUSTER_NAME', value: ${params.CLUSTER_NAME}], [$class: 'StringParameterValue', name: 'KOPS_STATE_STORE', value: ${params.KOPS_STATE_STORE}], [$class: 'StringParameterValue', name: 'DNS_ZONE_ID', value: ${params.DNS_ZONE_ID}]]
            copyArtifacts(projectName: 'aws-kops', selector: specific("${built.number}"));
            } 
          }

        
      }
        
    }

 post {
  always {
    withKubeConfig([credentialsId: 'kubeconfig-onprem', serverUrl: 'https://10.3.100.81:6443']) {
    sh '''
      kubectl get pods -n nuodb
      echo "Doing cleanup"
      cd deploy/
      kubectl delete -n $OPERATOR_NAMESPACE -f role.yaml || true
      kubectl delete -n $OPERATOR_NAMESPACE -f role_binding.yaml || true
      kubectl delete -n $OPERATOR_NAMESPACE -f service_account.yaml || true
      kubectl delete -n $OPERATOR_NAMESPACE -f local-disk-class.yaml || true
      kubectl delete -n $OPERATOR_NAMESPACE -f operator.yaml || true
      kubectl delete -f crds/nuodb_v2alpha1_nuodb_cr.yaml -n $OPERATOR_NAMESPACE || true
      kubectl delete -f crds/nuodb_v2alpha1_nuodbycsbwl_cr.yaml -n $OPERATOR_NAMESPACE || true
      kubectl delete -f crds/nuodb_v2alpha1_nuodbinsightsserver_cr.yaml -n $OPERATOR_NAMESPACE || true
      kubectl get pods -n $OPERATOR_NAMESPACE  
      kubectl delete configmap nuodb-lic-configmap -n $OPERATOR_NAMESPACE || true
      echo "delete the Custom Resource to deploy NuoDB..."

      kubectl delete -f crds/nuodb_v2alpha1_nuodb_crd.yaml || true
      kubectl delete -f crds/nuodb_v2alpha1_nuodbinsightsserver_crd.yaml || true
      kubectl delete -f crds/nuodb_v2alpha1_nuodbycsbwl_crd.yaml || true

    ''' 
    }
   
  }
  failure {
  // notify users when the Pipeline fails
    mail to: 'ashukla@nuodb.com',
    subject: "Failed Pipeline: ${currentBuild.fullDisplayName}",
    body: "Something is wrong with ${env.BUILD_URL}"
  }
  success {
    echo "Build success"
  }
 }

}
  