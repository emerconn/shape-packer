#!/bin/bash

# ==========================================
# Configuration
# ==========================================
PROJECT_ID="basic-bison-138323"
REGION="us-central1"
IMAGE="us-central1-docker.pkg.dev/basic-bison-138323/ghcr-proxy/emerconn/polygon-packer:v0.0.8"
SERVICE_ACCOUNT="817010668749-compute@developer.gserviceaccount.com"

# The static arguments (matching your YAML)
ARG2="3"
ARG3="6"

gcloud config set project "${PROJECT_ID}"

# ==========================================
# Execution Loop
# ==========================================
for i in {1..40}; do
  JOB_NAME="polygon-packer-$i-$ARG2-$ARG3"
  
  echo "======================================"
  echo "Creating: $JOB_NAME"
  echo "======================================"

  # 1. Strictly CREATE the job (will fail if it already exists)
  if ! gcloud run jobs create "${JOB_NAME}" \
    --project="${PROJECT_ID}" \
    --region="${REGION}" \
    --image="${IMAGE}" \
    --args="${i},${ARG2},${ARG3}" \
    --set-env-vars="OUTPUT_DIR=/mnt/data" \
    --cpu="8" \
    --memory="4Gi" \
    --add-volume="name=gcs-1,type=cloud-storage,bucket=polygon-picker" \
    --add-volume-mount="volume=gcs-1,mount-path=/mnt/data" \
    --max-retries="0" \
    --task-timeout="604800s" \
    --execution-environment="gen2" \
    --tasks="1" \
    --service-account="${SERVICE_ACCOUNT}"; then
      
      echo "ERROR: Failed to create $JOB_NAME. It likely already exists."
      echo "Aborting script to prevent unintended behavior."
      exit 1
  fi

  # 2. Execute the job
  echo "Starting execution for $JOB_NAME..."
  gcloud run jobs execute "${JOB_NAME}" \
    --project="${PROJECT_ID}" \
    --region="${REGION}" \
    --async

  echo ""
done

echo "All 40 jobs have been successfully created and triggered!"