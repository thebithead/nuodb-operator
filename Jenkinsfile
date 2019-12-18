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
                operator-sdk test local ./test/e2e --namespace nuodb  --go-test-flags "-timeout 1200s" --verbose --image $NUODB_OP_IMAGE:${GIT_COMMIT}

                kubectl get pods -n nuodb

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
  