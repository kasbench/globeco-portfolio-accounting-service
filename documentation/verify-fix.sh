#!/bin/bash

echo "=========================================="
echo "Verifying WriteHeader Fix"
echo "=========================================="
echo ""

# Build the server
echo "Building server..."
go build -o ./server ./cmd/server
if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi
echo "✅ Build successful"
echo ""

echo "Starting server in background..."
./server > server.log 2>&1 &
SERVER_PID=$!
echo "Server PID: $SERVER_PID"

# Wait for server to start
echo "Waiting for server to start..."
sleep 3

# Check if server is running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "❌ Server failed to start"
    cat server.log
    rm -f ./server
    exit 1
fi
echo "✅ Server started"
echo ""

# Make test requests
echo "Making test requests..."
curl -s http://localhost:8087/health > /dev/null 2>&1
curl -s http://localhost:8087/api/v1/health > /dev/null 2>&1
curl -s http://localhost:8087/api/v1/transactions > /dev/null 2>&1
curl -s http://localhost:8087/api/v1/balances > /dev/null 2>&1
echo "✅ Requests completed"
echo ""

# Wait a moment for logs to flush
sleep 1

# Check for superfluous WriteHeader warnings
echo "Checking for superfluous WriteHeader warnings..."
WARNINGS=$(grep -c "superfluous response.WriteHeader" server.log || echo "0")

echo ""
echo "=========================================="
if [ "$WARNINGS" -eq "0" ]; then
    echo "✅ SUCCESS! No superfluous WriteHeader warnings found!"
else
    echo "❌ FAILED! Found $WARNINGS superfluous WriteHeader warnings"
    echo ""
    echo "Sample warnings:"
    grep "superfluous response.WriteHeader" server.log | head -5
fi
echo "=========================================="
echo ""

# Cleanup
echo "Stopping server..."
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
rm -f ./server

echo ""
echo "Full server log saved to: server.log"
echo "Review it with: cat server.log"
