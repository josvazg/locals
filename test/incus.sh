#!/usr/bin/env bash
NODE_NAME="test-$(date +%s)"
IMAGE=$1 # e.g., "images:archlinux" or "images:fedora/40"
shift

if [ -z "$IMAGE" ]; then
  echo "Usage: $0 <image_name> [<shell_script>]"
  exit 1
fi

echo "--- Launching Real $IMAGE System Container via Incus ---"

# 1. Launch the container (privileged so bind-mounts work)
incus launch "$IMAGE" "$NODE_NAME" -c security.privileged=true -c security.nesting=true --ephemeral 

# 2. Wait for systemd to boot
echo "Waiting for systemd..."
sleep 5 

# 3. Mount your current directory directly (No more file push!)
# This maps your local folder to /src inside the container
incus config device add "$NODE_NAME" project-src disk source="$(pwd)" path=/src

# 4. Install dependencies and run tests
SAFE_ARGS=$(printf "%q " "$@")
incus exec "$NODE_NAME" -- sh -c "/src/test/distro.sh $SAFE_ARGS"

# 5. Cleanup (optional)
incus delete -f "$NODE_NAME"
