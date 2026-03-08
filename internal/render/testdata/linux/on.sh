#!/usr/bin/env bash
set -Eeuo pipefail
LOCALS_DIR="/home/user/.config/locals"

# --- enable mkcerts ---
mkcert -install
echo "For CLI usage run"
echo "source <(locals env)"

# --- setup locals DNS ---
DNS_LISTEN="127.1.2.3"
RESOLV_CONF_LOCAL="${LOCALS_DIR}/resolv.patched.conf"
DNS_PID_FILE="$HOME/.config/locals/dns.pid"

function launch_dns() {
    if [ -f "$DNS_PID_FILE" ]; then
        DNS_PID=$(cat "$DNS_PID_FILE")
        if kill -0 "$DNS_PID" 2>/dev/null; then
            echo "⚠️ locals dns is already running (PID: $DNS_PID). Skipping start."
            return
        fi
        echo "🔄 Cleaning up stale PID file from previous crash..."
        rm "$DNS_PID_FILE"
    fi
    # note fallbacks are extracted by the not yet replace /etc/resolv.conf
    sudo nohup locals dns "$DNS_LISTEN" > /tmp/locals-dns.log 2>&1 &
    echo $! > "$DNS_PID_FILE"
    echo "✅ locals DNS started on $DNS_LISTEN (PID: $(cat $DNS_PID_FILE))"
}

function apply_dns_config() {
    if mountpoint -q "/etc/resolv.conf"; then
        echo "⚠️ /etc/resolv.conf already replaced. Skipping."
        return
    fi

    cat <<EOF > "$RESOLV_CONF_LOCAL"
nameserver 127.1.2.3
options edns0 trust-ad
EOF

    sudo mount --bind "$RESOLV_CONF_LOCAL" /etc/resolv.conf
    echo "🔒 /etc/resolv.conf mounted to redirect DNS queries to locals dns first"
}

launch_dns
apply_dns_config

# --- stop locals web proxy ---
WEB_PID_FILE="${LOCALS_DIR}/web.pid"
RULES_DIR="${LOCALS_DIR}/web"
BINARY_PATH="$(which locals)"

function launch_web() {
    mkdir -p "$RULES_DIR"
    if [ -f "$WEB_PID_FILE" ]; then
        OLD_PID=$(cat "$WEB_PID_FILE")
        if kill -0 "$OLD_PID" 2>/dev/null; then
            echo "⚠️ locals web is already running (PID: $OLD_PID)."
            return
        fi
    fi
    sudo nohup locals web "${RULES_DIR}" > /tmp/locals-web.log 2>&1 &
    echo $! > "$WEB_PID_FILE"
    echo "✅ Web proxy active on :443 (PID: $(cat $WEB_PID_FILE))"
}

launch_web
