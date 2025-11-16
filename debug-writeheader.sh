#!/bin/bash

# Debug WriteHeader Script
# This script runs the server with the debug middleware enabled
# to trace superfluous WriteHeader calls

echo "=========================================="
echo "Debug WriteHeader Middleware Test"
echo "=========================================="
echo ""
echo "This will start the server with debug middleware enabled."
echo "Watch for stack traces showing duplicate WriteHeader calls."
echo ""
echo "Press Ctrl+C to stop the server"
echo ""
echo "=========================================="
echo ""

# Build the server
echo "Building server..."
go build -o ./server ./cmd/server
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "Starting server with debug middleware..."
echo ""

# Run the server
./server

# Cleanup
rm -f ./server
