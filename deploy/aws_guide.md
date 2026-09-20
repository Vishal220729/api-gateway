# Complete AWS Deployment Guide: APIGatewayCore

This guide explains step-by-step how to deploy your **Go API Gateway + Redis Distributed Rate Limiter + 8-Page Web Dashboard** to Amazon Web Services (AWS).

---

## 📋 Recommended Architecture: AWS EC2 + Docker Compose

```
Internet Users & Clients
          │
          ▼
   [ AWS EC2 Instance ] (Public IP: e.g. 54.210.xx.xx)
   ┌────────────────────────────────────────────────────────┐
   │ Security Group: Ports 22 (SSH), 80, 443, 8080 (Gateway)│
   │                                                        │
   │  Docker Engine                                         │
   │  ┌────────────────────────┐   ┌─────────────────────┐  │
   │  │ api-gateway-core       │   │ api-gateway-redis   │  │
   │  │ (Go Binary + 8 Pages)  ├───┤ (Redis 7 In-Memory) │  │
   │  │ Port :8080             │   │ Port :6379          │  │
   │  └────────────────────────┘   └─────────────────────┘  │
   └────────────────────────────────────────────────────────┘
```

---

## 🚀 Step 1: Launch an EC2 Instance on AWS Console

1. Log into your [AWS Management Console](https://console.aws.amazon.com/).
2. Navigate to **EC2** &rarr; Click **Launch Instance**.
3. **Name**: `api-gateway-production`.
4. **Application and OS Images (Amazon Machine Image)**:
   - Select **Ubuntu** &rarr; **Ubuntu Server 24.04 LTS (HVM)** or **Ubuntu 22.04 LTS**.
5. **Instance Type**:
   - For Free Tier: **`t2.micro`** (1 vCPU, 1 GiB RAM).
   - For Production: **`t3.small`** or **`t3.medium`** (Recommended for high concurrency).
6. **Key Pair (Login)**:
   - Select an existing key pair or click **Create new key pair** (e.g. `gateway-key.pem`).
7. **Network Settings (Security Group)**:
   - Click **Edit** network settings.
   - Create a new Security Group: `api-gateway-sg`.
   - Add the following Inbound Rules:

| Type | Port Range | Protocol | Source | Purpose |
|---|---|---|---|---|
| **SSH** | `22` | TCP | `My IP` | Secure SSH Terminal Access |
| **HTTP** | `80` | TCP | `0.0.0.0/0` | Standard Web Traffic |
| **HTTPS** | `443` | TCP | `0.0.0.0/0` | Secure SSL Web Traffic |
| **Custom TCP** | `8080` | TCP | `0.0.0.0/0` | **APIGatewayCore Web Dashboard & API** |

8. **Advanced Details (User Data)**:
   - Scroll down to **Advanced Details** &rarr; find **User Data**.
   - Paste the contents of [`deploy/ec2_setup.sh`](file:///c:/Users/VISHALRR/.gemini/antigravity-ide/scratch/api-gateway/deploy/ec2_setup.sh):

```bash
#!/usr/bin/env bash
set -euo pipefail

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
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 8080/tcp
ufw --force enable

mkdir -p /opt/api-gateway
```

9. Click **Launch Instance**!

---

## 📦 Step 2: Upload Your Code & Start Containers

Once your instance status is **Running**:

### 1. Find your Public IP:
In the EC2 instances list, copy the **Public IPv4 address** (e.g. `54.210.45.123`).

### 2. Copy the Codebase to your EC2 instance:
From your local Windows terminal / PowerShell, upload your project folder to EC2 using `scp`:

```powershell
# In PowerShell:
scp -i "path\to\gateway-key.pem" -r ./* ubuntu@<EC2-PUBLIC-IP>:/opt/api-gateway/
```

*(Alternatively, push your code to GitHub and clone it directly on the server: `git clone <your-repo-url> /opt/api-gateway`)*

### 3. Connect via SSH:
```bash
ssh -i "path/to/gateway-key.pem" ubuntu@<EC2-PUBLIC-IP>
```

### 4. Build and Start the Containers:
```bash
cd /opt/api-gateway
sudo docker compose up -d --build
```

Verify that both containers are running and healthy:
```bash
sudo docker compose ps
```
Output:
```
NAME                IMAGE               COMMAND                  SERVICE             STATUS              PORTS
api-gateway-core    api-gateway-core    "/app/api-gateway"       gateway             running (healthy)   0.0.0.0:8080->8080/tcp
api-gateway-redis   redis:7-alpine      "docker-entrypoint.s…"   redis               running (healthy)   0.0.0.0:6379->6379/tcp
```

---

## 🌐 Step 3: Access Your Live Dashboard

Open your browser and navigate to:
```
http://<EC2-PUBLIC-IP>:8080/intro.html
```

Or any of the dedicated pages:
- **Overview**: `http://<EC2-PUBLIC-IP>:8080/index.html`
- **Traffic**: `http://<EC2-PUBLIC-IP>:8080/traffic.html`
- **Security**: `http://<EC2-PUBLIC-IP>:8080/security.html`
- **API Keys**: `http://<EC2-PUBLIC-IP>:8080/apikeys.html`
- **Routes**: `http://<EC2-PUBLIC-IP>:8080/routes.html`
- **Cloud Hub**: `http://<EC2-PUBLIC-IP>:8080/cloud.html`
- **Chaos**: `http://<EC2-PUBLIC-IP>:8080/chaos.html`

---

## 🔒 Step 4 (Optional): Add Custom Domain & Free SSL (HTTPS)

To serve your gateway on a standard domain (e.g., `api.yourdomain.com`) with free SSL:

### 1. Install Nginx & Certbot on EC2:
```bash
sudo apt-get install -y nginx certbot python3-certbot-nginx
```

### 2. Configure Nginx Reverse Proxy:
Create `/etc/nginx/sites-available/api-gateway`:
```nginx
server {
    listen 80;
    server_name api.yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable site:
```bash
sudo ln -s /etc/nginx/sites-available/api-gateway /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

### 3. Generate Free Let's Encrypt SSL Certificate:
```bash
sudo certbot --nginx -d api.yourdomain.com
```

Now your gateway is live at: `https://api.yourdomain.com/intro.html`!
