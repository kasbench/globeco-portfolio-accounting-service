#!/bin/bash
set -euo pipefail

NAMESPACE=globeco
POD_NAME=tcpdump-debug

# Apply the debug pod manifest
kubectl apply -f tcpdump-debug.yaml -n $NAMESPACE

echo "Waiting for debug pod to be ready..."
kubectl wait --for=condition=Ready pod/$POD_NAME -n $NAMESPACE --timeout=60s

echo "Starting tcpdump in the debug pod. Press Ctrl+C to stop."
kubectl exec -it $POD_NAME -n $NAMESPACE -- tcpdump -A -s 512 'tcp port 5701'

echo "Deleting debug pod..."
kubectl delete pod $POD_NAME -n $NAMESPACE 