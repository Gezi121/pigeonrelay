#!/bin/sh
echo "========================================="
echo " PigeonRelay Speed Test Client"
echo " $(date -u +"%Y-%m-%d %H:%M:%S UTC")"
echo " Architecture: $(uname -m)"
echo " Python: $(python3 --version 2>&1)"
echo "========================================="
echo ""

exec python3 /app/client.py
