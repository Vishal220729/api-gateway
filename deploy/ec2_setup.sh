#!/usr/bin/env bash
# ==============================================================================
# AWS EC2 1-Click Zero-Touch Setup Script (User Data / Cloud-Init)
# Supports: Ubuntu 22.04 / 24.04 LTS & Debian 12
# ==============================================================================

set -euo pipefail

echo "=========================================================="
echo " Starting APIGatewayCore Deployment on AWS EC2"
echo "=========================================================="

# 1. Update and install prerequisites
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y ca-certificates curl gnupg lsb-release git ufw

# 2. Install official Docker and Docker Compose Plugin
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null

apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# 3. Enable and start Docker
systemctl enable docker
systemctl start docker

# 4. Configure UFW Firewall
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'SSH'
ufw allow 80/tcp comment 'HTTP'
ufw allow 443/tcp comment 'HTTPS'
ufw allow 8080/tcp comment 'APIGateway'
ufw --force enable

# 5. Create deployment directory
DEPLOY_DIR="/opt/api-gateway"
mkdir -p "$DEPLOY_DIR"
cd "$DEPLOY_DIR"

echo "Directory created at $DEPLOY_DIR"
echo "To deploy your codebase:"
echo "1. Upload your code files to $DEPLOY_DIR"
echo "2. Run: cd $DEPLOY_DIR && docker compose up -d --build"
echo "=========================================================="
echo " Gateway will be available at: http://<EC2-PUBLIC-IP>:8080/intro.html"
echo "=========================================================="
