#!/usr/bin/env bash
set -Eeuo pipefail
LOCALS_DIR="/home/user/.config/locals"

# --- remove web endpoint ---
DOMAIN=$1
rm -f "${LOCALS_DIR}/web/${DOMAIN}.json"
rm -f "${LOCALS_DIR}/certs/${DOMAIN}.pem" "${LOCALS_DIR}/certs/${DOMAIN}-key.pem"
echo "⏹️ Removed access to ${DOMAIN}"
