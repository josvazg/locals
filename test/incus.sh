#!/usr/bin/env bash

set -euo pipefail

NODE_NAME="test-$(date +%s)"
IMAGE=$1 # e.g., "images:archlinux" or "images:fedora/40"
shift

if [ -z "$IMAGE" ]; then
  echo "Usage: $0 <image_name> [<shell_script>]"
  exit 1
fi

echo "--- Launching Real $IMAGE System Container via Incus ---"

incus launch "$IMAGE" "$NODE_NAME" -c security.privileged=true -c security.nesting=true --ephemeral 

echo "Waiting for systemd..."
sleep 5 

# This maps your local folder to /src inside the container
incus config device add "$NODE_NAME" project-src disk source="$(pwd)" path=/src

SAFE_ARGS=$(printf "%q " "$@")
incus exec "$NODE_NAME" -- sh -c "/src/test/distro.sh $SAFE_ARGS"
EXIT_CODE=$?

incus delete -f "$NODE_NAME"

exit "${EXIT_CODE}"
