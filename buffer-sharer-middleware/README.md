# Buffer Sharer Middleware

Relay server for Buffer Sharer application. Connects controllers and clients via room codes.

## Quick Deploy to VPS

### Option 1: One-liner (Ubuntu/Debian)

```bash
# On your VPS
curl -sSL https://raw.githubusercontent.com/buffer-sharer/middleware/main/deploy.sh | sudo bash
```

### Option 2: Manual Installation

1. **Build on your machine:**
```bash
# For Linux x86_64 VPS
make build-linux

# For Linux ARM64 (Oracle Cloud Free Tier, etc.)
make build-linux-arm
```

2. **Copy to VPS:**
```bash
scp buffer-sharer-middleware-linux-amd64 root@YOUR_VPS_IP:/tmp/
scp buffer-sharer-middleware.service root@YOUR_VPS_IP:/tmp/
scp deploy.sh root@YOUR_VPS_IP:/tmp/
```

3. **Install on VPS:**
```bash
ssh root@YOUR_VPS_IP
cd /tmp
chmod +x deploy.sh
./deploy.sh 8080
```

### Option 3: Build on VPS

```bash
# Install Go
apt update && apt install -y golang-go

# Clone/copy files
git clone https://github.com/buffer-sharer/middleware.git
cd middleware

# Build and install
go build -o buffer-sharer-middleware .
sudo mv buffer-sharer-middleware /usr/local/bin/
sudo cp buffer-sharer-middleware.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now buffer-sharer-middleware
```

## Configuration

### Change port

Edit the service file:
```bash
sudo systemctl edit buffer-sharer-middleware
```

Add:
```ini
[Service]
ExecStart=
ExecStart=/usr/local/bin/buffer-sharer-middleware -port 9999
```

Then restart:
```bash
sudo systemctl restart buffer-sharer-middleware
```

### Firewall (UFW)

```bash
sudo ufw allow 8080/tcp
sudo ufw enable
```

### Firewall (firewalld)

```bash
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
```

## Management Commands

```bash
# Status
sudo systemctl status buffer-sharer-middleware

# Logs (follow)
sudo journalctl -u buffer-sharer-middleware -f

# Restart
sudo systemctl restart buffer-sharer-middleware

# Stop
sudo systemctl stop buffer-sharer-middleware

# Uninstall
sudo systemctl stop buffer-sharer-middleware
sudo systemctl disable buffer-sharer-middleware
sudo rm /etc/systemd/system/buffer-sharer-middleware.service
sudo rm /usr/local/bin/buffer-sharer-middleware
sudo systemctl daemon-reload
```

## Cloud Provider Examples

### Oracle Cloud (Free Tier)

1. Create free VM (ARM-based is cheaper)
2. Add ingress rule: Protocol TCP, Port 8080
3. Deploy using instructions above

### DigitalOcean

1. Create cheapest droplet ($4/mo)
2. Deploy using instructions above
3. Firewall is usually open by default

### AWS EC2

1. Create t2.micro (free tier eligible)
2. Security Group: Allow TCP 8080 from 0.0.0.0/0
3. Deploy using instructions above

### Hetzner Cloud

1. Create CX11 (~€3/mo)
2. Deploy using instructions above

## Usage in App

In Buffer Sharer app settings:
- **Host:** Your VPS IP or domain
- **Port:** 8080 (or your configured port)

Example:
- Host: `123.45.67.89` or `middleware.your-domain.com`
- Port: `8080`

## How It Works

```
┌─────────────┐                  ┌─────────────────────┐                  ┌──────────┐
│ Controller  │ ───TCP:8080───> │    Middleware       │ <───TCP:8080─── │  Client  │
│  (creates   │                  │  (relays messages)  │                  │  (joins  │
│   room)     │                  │                     │                  │   room)  │
└─────────────┘                  └─────────────────────┘                  └──────────┘
      │                                   │                                     │
      │  1. Connect as controller         │                                     │
      │  ─────────────────────────────>   │                                     │
      │  2. Receive room code (ABC123)    │                                     │
      │  <─────────────────────────────   │                                     │
      │                                   │   3. Connect with code ABC123       │
      │                                   │   <────────────────────────────     │
      │                                   │   4. Auth success                   │
      │                                   │   ────────────────────────────>     │
      │                                   │                                     │
      │  5. Send screenshot/text          │                                     │
      │  ─────────────────────────────>   │   6. Relay to client               │
      │                                   │   ────────────────────────────>     │
```

## Security Notes

- The middleware only relays messages, it doesn't store them
- Room codes expire after 24 hours of inactivity
- Consider using a reverse proxy (nginx) with SSL for production
- Limit access by IP if needed

## SSL with Nginx (Optional)

```nginx
stream {
    upstream middleware {
        server 127.0.0.1:8080;
    }

    server {
        listen 8443 ssl;
        proxy_pass middleware;

        ssl_certificate /etc/letsencrypt/live/your-domain/fullchain.pem;
        ssl_certificate_key /etc/letsencrypt/live/your-domain/privkey.pem;
    }
}
```

## License

MIT
