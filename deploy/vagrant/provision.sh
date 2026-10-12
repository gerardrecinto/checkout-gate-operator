#!/usr/bin/env bash
# Installs NGINX with a self-signed TLS site, runs it as the unprivileged www-data
# user, applies a few sysctls, and checks the result. Exits non-zero if a check fails.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update -qq
apt-get install -y -qq nginx openssl curl

install -d -m 0750 /etc/nginx/tls
openssl req -x509 -newkey rsa:2048 -nodes -days 30 \
  -keyout /etc/nginx/tls/edge.key -out /etc/nginx/tls/edge.crt -subj "/CN=edge-lab" 2>/dev/null
chmod 0640 /etc/nginx/tls/edge.key && chgrp www-data /etc/nginx/tls/edge.key

cat > /etc/nginx/conf.d/edge.conf <<'CONF'
server {
  listen 443 ssl;
  server_tokens off;
  ssl_certificate     /etc/nginx/tls/edge.crt;
  ssl_certificate_key /etc/nginx/tls/edge.key;
  ssl_protocols       TLSv1.2 TLSv1.3;
  location /healthz { return 200 "ok\n"; }
}
CONF

cat > /etc/sysctl.d/90-edge.conf <<'CONF'
net.ipv4.conf.all.rp_filter = 1
net.ipv4.tcp_syncookies = 1
net.ipv4.conf.all.accept_redirects = 0
CONF
sysctl --system >/dev/null

nginx -t
systemctl enable --now nginx
systemctl reload nginx

# Checks
[ "$(curl -ks https://localhost/healthz)" = ok ] || { echo "FAIL healthz"; exit 1; }
curl -ksI https://localhost/healthz | grep -qi '^server: nginx$' || { echo "FAIL server header leaks version"; exit 1; }
ps -o user= -C nginx | sort -u | tr '\n' ' ' | grep -q www-data || { echo "FAIL no nginx worker as www-data"; exit 1; }
[ "$(sysctl -n net.ipv4.tcp_syncookies)" = 1 ] || { echo "FAIL syncookies"; exit 1; }
echo "provision checks passed on $(uname -r), kvm guest: $(systemd-detect-virt)"
