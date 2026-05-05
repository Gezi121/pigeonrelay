#!/usr/bin/env bash
set -e

echo "=== PigeonRelay One-Click Deploy ==="

mkdir -p ~/pigeonrelay/data
cd ~/pigeonrelay

# Generate persistent JWT secret (survives redeploys)
JWT_FILE=./data/.jwt_secret
if [ ! -f "$JWT_FILE" ]; then
  openssl rand -hex 32 > "$JWT_FILE"
fi
JWT_SECRET=$(cat "$JWT_FILE")

cat > docker-compose.yml <<'YML'
services:
  pigeonrelay:
    image: ghcr.io/gezi121/pigeonrelay:latest
    network_mode: host
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/config.yaml:ro
    environment:
      - DB_PATH=/app/data/pigeonrelay.db
      - JWT_SECRET=__JWT_SECRET__
      - PORT=3214
      - STATIC_DIR=/app/static
      - ADMIN_USER=admin
      - ADMIN_PASS=admin123
      - BACKUP_RETENTION_COUNT=2
      - LATENCY_TOKEN=change-me
      # CF 优选（可选，也可在 Settings 页面配置）
      # - CF_API_TOKEN=
      # - CF_ZONE_ID=
    restart: always
YML
sed -i "s/__JWT_SECRET__/$JWT_SECRET/" docker-compose.yml

if [ ! -f config.yaml ]; then
  cat > config.yaml <<'YAML'
db_path: /app/data/pigeonrelay.db
jwt_secret: ""
port: "3214"
backup_retention_count: 2
admin_user: admin
admin_pass: admin123
notification_webhook: ""
YAML
fi

docker compose pull
docker compose up -d

echo ""
echo "=== PigeonRelay is running ==="
echo "  URL:   http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'YOUR_IP'):3214"
echo "  Login: admin / admin123"
echo ""
echo "  Stop:  cd ~/pigeonrelay && docker compose down"
