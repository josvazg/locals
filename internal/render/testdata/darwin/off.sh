#!/usr/bin/env bash
set -Eeuo pipefail
LOCALS_DIR="/home/user/.config/locals"

# --- remove locals DNS ---
DNS_LISTEN="127.1.2.3"
DNS_PID_FILE="${LOCALS_DIR}/dns.pid"
# This removes the locals dns redirect
sudo ifconfig lo0 -alias "${DNS_LISTEN}" || echo "locals dns address already unmapped"
sudo rm -f /etc/resolver/locals

if [ -f "$DNS_PID_FILE" ]; then
    DNS_PID=$(cat "$DNS_PID_FILE")
    if kill -0 "$DNS_PID" 2>/dev/null; then
        kill "$DNS_PID"
        sleep 0.5
        if kill -0 "$DNS_PID" 2>/dev/null; then
            sudo kill -9 "$PID"
        fi

        echo "🛑 Terminated locals dns (PID: $DNS_PID)"
    else
        echo "⚠️ PID file exists but process $DNS_PID is already dead."
    fi
    rm "$DNS_PID_FILE"
else
    echo "ℹ️ No DNS PID file found. Nothing to kill."
fi

# --- deactivate locals web proxy ---
WEB_PID_FILE="$HOME/.config/locals/web.pid"

if [ -f "$WEB_PID_FILE" ]; then
    PID=$(cat "$WEB_PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        kill "$PID"
        echo "🛑 Stopped web process ($PID)"
    else
        echo "⚠️ Web proxy was not running (stale PID file)."
    fi
    rm "$WEB_PID_FILE"
else
    echo "ℹ️ No web proxy PID file found."
fi

# --- disable mkcerts ---
mkcert -uninstall || echo "mkcert already uninstalled?"
