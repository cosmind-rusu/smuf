# Server Setup

This guide covers setting up `smuf-server` on a VPS for production use.

## Prerequisites

- A VPS with a public IP (any Linux distribution)
- A domain name pointing to your VPS (e.g., `smuf.yourdomain.com`)
- Ports 7000 (control) and 8080 (HTTP) accessible

## Basic setup

```bash
# Download the server binary
wget https://github.com/cosmind-rusu/smuf/releases/latest/download/smuf_0.3.0_Linux_x86_64.tar.gz
tar -xzf smuf_0.3.0_Linux_x86_64.tar.gz
sudo mv smuf-server /usr/local/bin/

# Create a config directory
mkdir -p /etc/smuf
```

## Configuration

Create `/etc/smuf/smuf-server.env`:

```env
SMUF_DOMAIN=smuf.yourdomain.com
SMUF_AUTH_TOKEN=<generate-with-openssl-rand-hex-32>
SMUF_HTTP_PORT=8080
SMUF_CONTROL_PORT=7000
SMUF_MAX_CONNS_PER_IP=10
```

## Systemd service

Create `/etc/systemd/system/smuf-server.service`:

```ini
[Unit]
Description=smuf HTTP tunnel server
After=network.target

[Service]
Type=simple
EnvironmentFile=/etc/smuf/smuf-server.env
ExecStart=/usr/local/bin/smuf-server
Restart=always
RestartSec=5
User=nobody
Group=nogroup

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable smuf-server
sudo systemctl start smuf-server
sudo systemctl status smuf-server
```

## HTTPS with Let's Encrypt (auto)

Set these environment variables to enable automatic TLS:

```env
SMUF_HTTPS=true
SMUF_HTTPS_PORT=443
SMUF_ACME_EMAIL=your@email.com
```

The server will automatically obtain and renew certificates from Let's Encrypt.

> **Note:** Port 80 must also be accessible for the ACME HTTP-01 challenge.

## Reverse proxy (Nginx)

If you prefer to terminate TLS at a reverse proxy:

```nginx
server {
    listen 443 ssl;
    server_name smuf.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Security checklist

- [ ] Generate a strong auth token (`openssl rand -hex 32`)
- [ ] Set `SMUF_MAX_CONNS_PER_IP` to prevent abuse
- [ ] Use HTTPS (auto Let's Encrypt or reverse proxy)
- [ ] Run as non-root user (systemd `User=nobody`)
- [ ] Configure firewall (allow only ports 7000, 8080, 443, 80)
- [ ] Keep the server updated to the latest version
