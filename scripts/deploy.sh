#!/bin/bash

# ==========================================
# Configuration
# ==========================================
PROJECT_ID="basic-bison-138323"
REGION="us-east5" # Columbus
IMAGE="us-central1-docker.pkg.dev/basic-bison-138323/ghcr-proxy/emerconn/shape-packer:v0.2.0"
SERVICE_ACCOUNT="817010668749-compute@developer.gserviceaccount.com"

COUNT_START=22
COUNT_END=50
INNER_SIDES="0"
OUTER_SIDES="0"

gcloud config set project "${PROJECT_ID}"

# ==========================================
# Execution Loop
# ==========================================
for i in $(seq ${COUNT_START} ${COUNT_END}); do
  JOB_NAME="shape-packer-$i-$INNER_SIDES-$OUTER_SIDES"
  
  echo "======================================"
  echo "Processing: $JOB_NAME"
  echo "======================================"

  # 1. Generate a clean YAML spec
  # This bypasses all gcloud flag parsing bugs
  cat <<EOF > temp-job.yaml
apiVersion: run.googleapis.com/v1
kind: Job
metadata:
  name: ${JOB_NAME}
spec:
  template:
    spec:
      taskCount: 1
      template:
        spec:
          containers:
          - image: ${IMAGE}
            args:
            - "--inner-count=${i}"
            - "--inner-sides=${INNER_SIDES}"
            - "--outer-sides=${OUTER_SIDES}"
            env:
            - name: GCP_BUCKET
              value: polygon-picker
            - name: FIRESTORE_PROJECT
              value: ${PROJECT_ID}
            - name: FIRESTORE_DATABASE
              value: shape-packer
            resources:
              limits:
                cpu: 8000m
                memory: 4Gi
          maxRetries: 0
          timeoutSeconds: 604800
          serviceAccountName: ${SERVICE_ACCOUNT}
EOF

  # 2. Use 'replace' with '--force' to create or update
  # This is the standard way to deploy from a file
  if ! gcloud run jobs replace temp-job.yaml --region="${REGION}"; then
      echo "ERROR: Failed to deploy $JOB_NAME."
      rm temp-job.yaml
      exit 1
  fi
  sleep 5

  # 3. Execute asynchronously
  gcloud run jobs execute "${JOB_NAME}" --region="${REGION}" --async
  sleep 5

  echo ""
done

# Cleanup
rm temp-job.yaml
echo "All jobs have been processed and triggered!"