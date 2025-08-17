pipeline {
  agent any

  options {
    timestamps()
    ansiColor('xterm')
    disableConcurrentBuilds()
  }

  environment {
    APP_NAME       = 'hobom-event-processor'

    // Docker Hub
    REGISTRY       = 'docker.io'
    IMAGE_REPO     = '<hub-username>/<repo>'      // ex) hobom/hobom-event-processor
    TAG            = "${env.BUILD_NUMBER}"
    IMAGE          = "${REGISTRY}/${IMAGE_REPO}:${TAG}"
    REGISTRY_CRED  = 'dockerhub-cred'             // Jenkins creds (username/password or token)

    // k3s
    KUBE_CONFIG    = 'kubeconfig-cred-id'         // Jenkins creds (Secret file)
    K8S_NAMESPACE  = 'default'
    DEPLOY_NAME    = 'hobom-event-processor'
    CONTAINER_NAME = 'processor'
  }

  stages {
    stage('Checkout (with submodules)') {
      steps {
        // 서브모듈 반드시 포함!
        checkout([$class: 'GitSCM',
          branches: scm.branches,
          userRemoteConfigs: scm.userRemoteConfigs,
          extensions: [[
            $class: 'SubmoduleOption',
            recursiveSubmodules: true,
            trackingSubmodules: false,
            reference: '',
            timeout: 20
          ]]
        ])
        sh 'git --no-pager log -1 --pretty=oneline'
        sh 'git submodule status || true'
      }
    }

    // 선택: 테스트(원하면 유지, 없애도 OK)
    stage('Unit Test (optional)') {
      steps {
        // Jenkins 노드에 Go가 없다면 docker로 테스트
        sh '''
          docker run --rm -v "$PWD":/app -w /app golang:1.22-alpine \
            sh -lc "apk add --no-cache git && go test ./... -count=1"
        '''
      }
      post {
        always {
          junit allowEmptyResults: true, testResults: '**/TEST-*.xml'
        }
      }
    }

    stage('Docker Build & Push') {
      steps {
        withCredentials([usernamePassword(credentialsId: env.REGISTRY_CRED, usernameVariable: 'REG_USER', passwordVariable: 'REG_PASS')]) {
          sh """
            docker build -t ${IMAGE} .
            echo "$REG_PASS" | docker login ${REGISTRY} -u "$REG_USER" --password-stdin
            docker push ${IMAGE}
            docker logout ${REGISTRY}
          """
        }
      }
    }

    stage('Deploy to k3s (only develop)') {
      when { expression { env.BRANCH_NAME == 'develop' } }
      steps {
        withCredentials([file(credentialsId: env.KUBE_CONFIG, variable: 'KUBECONFIG_FILE')]) {
          sh """
            export KUBECONFIG="$KUBECONFIG_FILE"
            kubectl -n ${K8S_NAMESPACE} set image deployment/${DEPLOY_NAME} ${CONTAINER_NAME}=${IMAGE} --record
            kubectl -n ${K8S_NAMESPACE} rollout status deployment/${DEPLOY_NAME} --timeout=300s
          """
        }
      }
    }
  }

  post {
    success {
      echo "✅ Build #${env.BUILD_NUMBER} OK (${env.BRANCH_NAME})"
      script {
        if (env.BRANCH_NAME == 'develop') {
          echo "🚀 Deployed ${IMAGE} to k3s (deployment=${DEPLOY_NAME}, container=${CONTAINER_NAME})"
        }
      }
    }
    failure {
      echo "❌ Build failed (${env.BRANCH_NAME})"
    }
  }
}
