#!/usr/bin/env bash
set -Eeuo pipefail
LOCALS_DIR="/home/user/.config/locals"

# --- add web endpoint ---
DOMAIN=$1
TARGET_URL=$2
mkcert -cert-file "${LOCALS_DIR}/certs/${DOMAIN}.pem" \
  -key-file "${LOCALS_DIR}/certs/${DOMAIN}-key.pem" \
  "${DOMAIN}" "*.locals" localhost 127.0.0.1 > /dev/null

mkdir -p "${LOCALS_DIR}/web"
cat <<EOF > "${LOCALS_DIR}/web/${DOMAIN}.json"
{
  "url": "${DOMAIN}",
  "endpoint": "${TARGET_URL}",
  "cert": "${LOCALS_DIR}/certs/${DOMAIN}.pem",
  "key": "${LOCALS_DIR}/certs/${DOMAIN}-key.pem"
}
EOF
echo "▶️ Added access to ${DOMAIN} -> ${TARGET_URL}"
