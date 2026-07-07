#!/bin/bash

# Test script to trigger requests and observe WriteHeader behavior

echo "Testing various endpoints to trigger WriteHeader calls..."
echo ""

BASE_URL="http://localhost:8087"

echo "1. Testing health endpoint..."
curl -s "$BASE_URL/health" > /dev/null
echo "   Done"
echo ""

echo "2. Testing API v1 health endpoint..."
curl -s "$BASE_URL/api/v1/health" > /dev/null
echo "   Done"
echo ""

echo "3. Testing transactions endpoint (should trigger the issue)..."
curl -s "$BASE_URL/api/v1/transactions" > /dev/null
echo "   Done"
echo ""

echo "4. Testing balances endpoint..."
curl -s "$BASE_URL/api/v1/balances" > /dev/null
echo "   Done"
echo ""

echo "5. Testing metrics endpoint..."
curl -s "$BASE_URL/metrics" > /dev/null
echo "   Done"
echo ""

echo "=========================================="
echo "Check the server logs for stack traces"
echo "Look for requests with 'WriteHeader called X times' where X > 1"
echo "=========================================="
