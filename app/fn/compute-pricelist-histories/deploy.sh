#! /bin/bash

# us-central1 because us-east1 does not fucking have go111 runtime yet

# deploying func
gcloud functions deploy ComputePricelistHistories \
    --runtime go111 \
    --trigger-topic computePricelistHistories \
    --source . \
    --memory 512MB \
    --region us-central1 \
    --timeout 120s