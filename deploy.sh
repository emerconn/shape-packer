#!/bin/bash

# ==========================================
# Configuration
# ==========================================
PROJECT_ID="basic-bison-138323"
REGION="us-central1"
IMAGE="us-central1-docker.pkg.dev/basic-bison-138323/ghcr-proxy/emerconn/polygon-packer:v0.0.8"
SERVICE_ACCOUNT="817010668749-compute@developer.gserviceaccount.com"

ARG2="3"
ARG3="6"

gcloud config set project "${PROJECT_ID}"

# ==========================================
# Execution Loop
# ==========================================
for i in {1..40}; do
  JOB_NAME="polygon-packer-$i-$ARG2-$ARG3"
  
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
            - "${i}"
            - "${ARG2}"
            - "${ARG3}"
            env:
            - name: OUTPUT_DIR
              value: /mnt/data
            resources:
              limits:
                cpu: 8000m
                memory: 4Gi
            volumeMounts:
            - name: gcs-1
              mountPath: /mnt/data
          volumes:
          - name: gcs-1
            csi:
              driver: gcsfuse.run.googleapis.com
              volumeAttributes:
                bucketName: polygon-picker
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

  # 3. Execute asynchronously
  echo "Starting execution for $JOB_NAME..."
  gcloud run jobs execute "${JOB_NAME}" --region="${REGION}" --async

  echo ""
done

# Cleanup
rm temp-job.yaml
echo "All 40 jobs have been processed and triggered!"