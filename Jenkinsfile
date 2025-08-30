pipeline {
  agent any

  options {
    timestamps()
    disableConcurrentBuilds()
  }

  environment {
    // Docker Hub
    REGISTRY       = 'docker.io'
    IMAGE_REPO     = 'jjockrod/hobom-system'
    SERVICE_NAME   = 'dev-hobom-event-processor'
    IMAGE_TAG      = "${REGISTRY}/${IMAGE_REPO}:${SERVICE_NAME}-${env.BUILD_NUMBER}"
    IMAGE_LATEST   = "${REGISTRY}/${IMAGE_REPO}:${SERVICE_NAME}-latest"
    REGISTRY_CRED  = 'dockerhub-cred'        // Docker Hub (push)
    READ_CRED_ID   = 'dockerhub-readonly'    // Remote pull (private)

    // Remote server
    APP_NAME       = 'dev-hobom-event-processor'
    DEPLOY_HOST    = 'ishisha.iptime.org'
    DEPLOY_PORT    = '22223'
    DEPLOY_USER    = 'infra-admin'
    SSH_CRED_ID    = 'deploy-ssh-key'

    // Runtime
    ENV_PATH       = '/etc/hobom-dev/dev-hobom-event-processor/.env'
    HOST_PORT      = '8082'
    CONTAINER_PORT = '8082'
  }

  stages {

    stage('Checkout') {
      steps {
        checkout scm
        sh '''
          set -eux
          git config --global --add safe.directory "$WORKSPACE" || true
          git submodule sync --recursive || true
          git submodule update --init --recursive || true
        '''
      }
    }

    stage('Build & Push Image (Docker)') {
      steps {
        withCredentials([usernamePassword(credentialsId: env.REGISTRY_CRED, usernameVariable: 'REG_USER', passwordVariable: 'REG_PASS')]) {
          sh '''
            set -eu
            export DOCKER_BUILDKIT=1

            # 로그인 (비밀 마스킹)
            set +x
            echo "$REG_PASS" | docker login "$REGISTRY" -u "$REG_USER" --password-stdin
            set -x

            docker build -t "${IMAGE_TAG}" -t "${IMAGE_LATEST}" .
            docker push "${IMAGE_TAG}"
            docker push "${IMAGE_LATEST}"
          '''
        }
      }
    }

    stage('Deploy container to server') {
      when { anyOf { branch 'develop'; branch 'main' } }
      steps {
        sshagent (credentials: [env.SSH_CRED_ID]) {
          withCredentials([usernamePassword(credentialsId: env.READ_CRED_ID, usernameVariable: 'PULL_USER', passwordVariable: 'PULL_PASS')]) {
            sh '''
set -eux

ssh -o StrictHostKeyChecking=no -p "$DEPLOY_PORT" "$DEPLOY_USER@$DEPLOY_HOST" \
  APP_NAME="$APP_NAME" \
  IMAGE="$IMAGE_LATEST" \
  CONTAINER="$APP_NAME" \
  ENV_PATH="$ENV_PATH" \
  HOST_PORT="$HOST_PORT" \
  CONTAINER_PORT="$CONTAINER_PORT" \
  PULL_USER="$PULL_USER" \
  PULL_PASS="$PULL_PASS" \
  bash -s <<'EOS'
set -euo pipefail
echo "[REMOTE] Deploying $APP_NAME with image $IMAGE"

# docker 설치 확인
if ! command -v docker >/dev/null 2>&1; then
  echo "[REMOTE][ERROR] docker not found. Install docker and add $USER to docker group."
  exit 1
fi

# private pull 로그인 (비밀 미노출)
echo "$PULL_PASS" | docker login docker.io -u "$PULL_USER" --password-stdin

# .env 확인 (필수)
if [ ! -f "$ENV_PATH" ]; then
  echo "[REMOTE][ERROR] $ENV_PATH not found. Create it first."
  exit 1
fi

# 최신 이미지 pull + 컨테이너 교체
docker pull "$IMAGE" || (echo "[REMOTE][ERROR] docker pull failed" && exit 1)

if docker ps -a --format '{{.Names}}' | grep -w "$CONTAINER" >/dev/null 2>&1; then
  docker stop "$CONTAINER" || true
  docker rm "$CONTAINER" || true
fi

docker network create hobom-net || true
docker run -d --name "$CONTAINER" \
  --network hobom-net \
  --restart unless-stopped \
  --env-file "$ENV_PATH" \
  --add-host=host.docker.internal:host-gateway \
  -p "${HOST_PORT}:${CONTAINER_PORT}" \
  "$IMAGE"

docker ps --filter "name=$CONTAINER" --format "table {{.Names}}\\t{{.Image}}\\t{{.Status}}\\t{{.Ports}}"
EOS
            '''
          }
        }
      }
    }
  }

  post {
    success {
      echo "✅ Build #${env.BUILD_NUMBER} → pushed ${env.IMAGE_TAG} & deployed on ${env.DEPLOY_HOST}"
    }
    failure {
      echo "❌ Build failed (${env.BRANCH_NAME})"
    }
  }
}
