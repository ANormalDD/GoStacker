#!/usr/bin/env bash
set -euo pipefail

echo "Installing Python dependencies..."
python3 -m pip install --user -r "$(dirname "$0")/requirements.txt"

echo "Running pyclient help:"
python3 "$(dirname "$0")/pyclient.py" --help

echo "Done. To run client: python3 client/pyclient.py [register|login|create-room|ws] --backend http://localhost:8080"
