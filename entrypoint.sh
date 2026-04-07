#!/bin/bash
set -e

echo "🌱 Initializing Tendril..."
mkdir -p /app/data/dynamic_skills /app/logs

touch /app/data/.initialized
echo "✅ Initialization complete. Starting Tendril..."

# Use uvicorn with --reload for live development via volume mount
exec uvicorn src.main:app \
    --host 0.0.0.0 \
    --port 8080 \
    --reload \
    --reload-dir /app/src \
    --log-level info
