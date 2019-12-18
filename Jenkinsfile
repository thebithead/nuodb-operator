  pipeline {
    agent {
      docker {
          image 'quay.io/nuodb/nuodb-operator-dev:golang-test'
          args '-v /var/run/docker.sock:/var/run/docker.sock -u 0:0 '
      }
    }

    
    parameters{
      string(defaultValue: 'quay.io/nuodb/nuodb-operator-dev', description: 'The image tag to be build and pushed', name: 'NUODB_OP_IMAGE', trim: false)
      choice(choices: ['OnPrem', 'EKS', 'GKE', 'AKS', 'OCP 4.x'], description: 'Specify A Environment to use for tests', name: 'CLUSTER_ENV')
      string(defaultValue: 'nuodb', description: 'Namespace for e2e tests', name: 'OPERATOR_NAMESPACE', trim: true)
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


      stage('E2E With Given Environment') {
        when { expression { params.CLUSTER_ENV == 'OnPrem' }}
      steps {
              withKubeConfig([credentialsId: 'kubeconfig-onprem', serverUrl: 'https://10.3.100.81:6443']) {
                sh '''
                kubectl create $OPERATOR_NAMESPACE
                kubectl create secret docker-registry regcred --namespace=$OPERATOR_NAMESPACE --docker-server=quay.io --docker-username="nuodb+nuodbdev" --docker-password="RLT4418GQN01MVEUW9Q4I7P7ZZTQ1I7O9JZYNO3T8I7SX9WK0G4VK64MEAIKG3S5" --docker-email=""
                
                operator-sdk test local ./test/e2e --namespace $OPERATOR_NAMESPACE  --go-test-flags "-timeout 1200s" --verbose --image $NUODB_OP_IMAGE:${GIT_COMMIT}

                kubectl get pods -n $OPERATOR_NAMESPACE

                ''' 
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
      kubectl delete -n $OPERATOR_NAMESPACE -f role.yaml
      kubectl delete -n $OPERATOR_NAMESPACE -f role_binding.yaml
      kubectl delete -n $OPERATOR_NAMESPACE -f service_account.yaml
      kubectl delete -n $OPERATOR_NAMESPACE -f local-disk-class.yaml 
      kubectl delete -n $OPERATOR_NAMESPACE -f operator.yaml
      kubectl delete -f crds/nuodb_v2alpha1_nuodb_cr.yaml -n $OPERATOR_NAMESPACE
      kubectl delete -f crds/nuodb_v2alpha1_nuodbycsbwl_cr.yaml -n $OPERATOR_NAMESPACE
      kubectl delete -f crds/nuodb_v2alpha1_nuodbinsightsserver_cr.yaml -n $OPERATOR_NAMESPACE
      kubectl get pods -n $OPERATOR_NAMESPACE  
      kubectl delete configmap nuodb-lic-configmap -n $OPERATOR_NAMESPACE
      echo "delete the Custom Resource to deploy NuoDB..."

      kubectl delete -f crds/nuodb_v2alpha1_nuodb_crd.yaml
      kubectl delete -f crds/nuodb_v2alpha1_nuodbinsightsserver_crd.yaml
      kubectl delete -f crds/nuodb_v2alpha1_nuodbycsbwl_crd.yaml

    ''' 
    }
   
  }
  failure {
  // notify users when the Pipeline fails
    mail to: 'ashukla@nuodb.com',
    subject: "Failed Pipeline: ${currentBuild.fullDisplayName}",
    body: "Something is wrong with ${env.BUILD_URL}"
  }
 }
}
  