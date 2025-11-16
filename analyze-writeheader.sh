#!/bin/bash

# Analyze WriteHeader debug output
# Usage: ./debug-writeheader.sh 2>&1 | tee debug.log
#        Then run: ./analyze-writeheader.sh debug.log

if [ -z "$1" ]; then
    echo "Usage: $0 <log-file>"
    echo ""
    echo "First, capture the debug output:"
    echo "  ./debug-writeheader.sh 2>&1 | tee debug.log"
    echo ""
    echo "Then analyze it:"
    echo "  ./analyze-writeheader.sh debug.log"
    exit 1
fi

LOG_FILE="$1"

if [ ! -f "$LOG_FILE" ]; then
    echo "Error: Log file '$LOG_FILE' not found"
    exit 1
fi

echo "=========================================="
echo "WriteHeader Analysis Report"
echo "=========================================="
echo ""

echo "Requests with multiple WriteHeader calls:"
echo "------------------------------------------"
grep "WriteHeader called" "$LOG_FILE" | grep -v "WriteHeader called 1 times" | sort | uniq -c
echo ""

echo "Most common duplicate call patterns:"
echo "-------------------------------------"
grep -A 20 "WriteHeader.*called (2 time" "$LOG_FILE" | grep "go.opentelemetry.io\|github.com/kasbench" | sort | uniq -c | sort -rn | head -10
echo ""

echo "Endpoints affected:"
echo "-------------------"
grep "Finished request" "$LOG_FILE" | awk '{print $4}' | sort | uniq -c | sort -rn
echo ""

echo "=========================================="
echo "Full analysis complete. Check $LOG_FILE for detailed stack traces."
echo "=========================================="
