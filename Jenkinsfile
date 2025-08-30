pipeline {
  agent any

  options {
    timestamps()
    disableConcurrentBuilds()
  }

  environment {
    // ===== Docker Hub =====
    REGISTRY       = 'docker.io'
    IMAGE_REPO     = 'jjockrod/hobom-system'
    SERVICE_NAME   = 'dev-hobom-event-processor'
    IMAGE_TAG      = "${REGISTRY}/${IMAGE_REPO}:${SERVICE_NAME}-${env.BUILD_NUMBER}"
    IMAGE_LATEST   = "${REGISTRY}/${IMAGE_REPO}:${SERVICE_NAME}-latest"
    REGISTRY_CRED  = 'dockerhub-cred'
    READ_CRED_ID   = 'dockerhub-readonly'

    // ===== Remote server =====
    APP_NAME       = 'dev-hobom-event-processor'
    DEPLOY_HOST    = 'ishisha.iptime.org'
    DEPLOY_PORT    = '22223'
    DEPLOY_USER    = 'infra-admin'
    SSH_CRED_ID    = 'deploy-ssh-key'

    // ===== Runtime networking =====
    HOST_PORT       = '8082'
    CONTAINER_PORT  = '8082'

    // ===== Build target =====
    TARGET_PLATFORM = 'linux/amd64'

    // ===== (Optional) env-file on remote host =====
    ENV_PATH        = '/etc/hobom-dev/dev-hobom-event-processor/.env'

    GRPC_TARGET     = 'host.docker.internal:50051'
    REDIS_ADDR      = 'redis:6379'
    KAFKA_BROKERS   = 'kafka:9092'
  }

  stages {

    stage('Checkout (with submodules)') {
      steps {
        checkout([
          $class: 'GitSCM',
          branches: scm.branches,
          userRemoteConfigs: scm.userRemoteConfigs,
          extensions: [
            [$class: 'CloneOption', shallow: true, depth: 1, noTags: false, honorRefspec: true],
            [$class: 'SubmoduleOption',
              disableSubmodules: false,
              parentCredentials: true,
              recursiveSubmodules: true,
              trackingSubmodules: false,
              reference: '',
              shallow: true,
              depth: 1
            ]
          ]
        ])
        sh '''
          set -eux
          git submodule sync --recursive
          git submodule update --init --recursive --depth 1
        '''
      }
    }

    stage('Unit tests (Go, in Docker)') {
      steps {
        sh '''
          set -eux
          docker run --rm -v "$PWD":/src -w /src golang:1.22-alpine \
            sh -lc "apk add --no-cache git && go test ./..."
        '''
      }
    }

    stage('Build & Push Image') {
      steps {
        withCredentials([usernamePassword(credentialsId: env.REGISTRY_CRED, usernameVariable: 'REG_USER', passwordVariable: 'REG_PASS')]) {
          sh '''
            set -eu
            export DOCKER_BUILDKIT=1
            GIT_SHA=$(git rev-parse --short HEAD || echo local)

            # login (masked)
            set +x
            echo "$REG_PASS" | docker login "$REGISTRY" -u "$REG_USER" --password-stdin
            set -x

            docker build \
              --platform "${TARGET_PLATFORM}" \
              --build-arg VERSION="${BUILD_NUMBER}" \
              --build-arg COMMIT="${GIT_SHA}" \
              -t "${IMAGE_TAG}" -t "${IMAGE_LATEST}" .

            docker push "${IMAGE_TAG}"
            docker push "${IMAGE_LATEST}"
          '''
        }
      }
    }

    stage('Deploy to server') {
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
  GRPC_TARGET="$GRPC_TARGET" \
  REDIS_ADDR="$REDIS_ADDR" \
  KAFKA_BROKERS="$KAFKA_BROKERS" \
  bash -s <<'EOS'
set -euo pipefail
echo "[REMOTE] Deploying $APP_NAME with image $IMAGE"

# docker login (masked)
echo "$PULL_PASS" | docker login docker.io -u "$PULL_USER" --password-stdin

# pull latest image
docker pull "$IMAGE"

# stop & remove existing
docker rm -f "$CONTAINER" || true

# network
docker network create hobom-net || true

# env-file가 있으면 우선 사용, 없으면 개별 -e로 주입
if [ -f "$ENV_PATH" ]; then
  ENV_OPTS="--env-file $ENV_PATH"
else
  ENV_OPTS="-e GRPC_TARGET=$GRPC_TARGET -e REDIS_ADDR=$REDIS_ADDR -e KAFKA_BROKERS=$KAFKA_BROKERS -e PORT=$CONTAINER_PORT"
fi

docker run -d --name "$CONTAINER" \
  --network hobom-net \
  --restart unless-stopped \
  --add-host=host.docker.internal:host-gateway \
  $ENV_OPTS \
  -p "${HOST_PORT}:${CONTAINER_PORT}" \
  "$IMAGE"

docker ps --filter "name=$CONTAINER" --format "table {{.Names}}\\t{{.Image}}\\t{{.Status}}\\t{{.Ports}}"
EOS
            '''
          }
        }
      }
    }

    stage('Smoke check') {
      when { anyOf { branch 'develop'; branch 'main' } }
      steps {
        sshagent (credentials: [env.SSH_CRED_ID]) {
          sh """
            ssh -o StrictHostKeyChecking=no -p ${env.DEPLOY_PORT} ${env.DEPLOY_USER}@${env.DEPLOY_HOST} '
              curl -fsS http://localhost:${env.HOST_PORT}/health || true
            '
          """
        }
      }
    }
  }

  post {
    success { echo "✅ Go svc #${env.BUILD_NUMBER} → pushed ${env.IMAGE_TAG} & deployed" }
    failure { echo "❌ Build failed (${env.BRANCH_NAME})" }
  }
}
