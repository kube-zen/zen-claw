# Installing Zen Claw as a Systemd Service

## Prerequisites

1. **Build Zen Claw**:
   ```bash
   cd ~/git/zen-claw
   go build -o zen-claw .
   ```

2. **Configure API keys** in `~/.zen/zen-claw/config.yaml`:
   ```bash
   ./zen-claw config check
   ```

## Installation Options

### Option 1: Simple Installation (Recommended)

```bash
# Copy service file to systemd directory
sudo cp zen-claw-simple.service /etc/systemd/system/zen-claw.service

# Reload systemd
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable zen-claw

# Start service now
sudo systemctl start zen-claw

# Check status
sudo systemctl status zen-claw
```

### Option 2: Secure Installation (More restrictive)

```bash
# Copy service file to systemd directory
sudo cp zen-claw.service /etc/systemd/system/zen-claw.service

# Reload systemd
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable zen-claw

# Start service now
sudo systemctl start zen-claw

# Check status
sudo systemctl status zen-claw
```

## Service Management Commands

```bash
# Start service
sudo systemctl start zen-claw

# Stop service
sudo systemctl stop zen-claw

# Restart service
sudo systemctl restart zen-claw

# Check status
sudo systemctl status zen-claw

# View logs
sudo journalctl -u zen-claw -f

# Enable on boot
sudo systemctl enable zen-claw

# Disable on boot
sudo systemctl disable zen-claw
```

## Testing the Gateway

Once the service is running, test it with:

```bash
# Health check
curl http://localhost:8080/health

# Chat with AI (using default provider from config)
curl -X POST http://localhost:8080/chat \
  -d "message=Hello, what's your model?" \
  -d "provider=deepseek"

# Chat with specific model
curl -X POST http://localhost:8080/chat \
  -d "message=What can you do?" \
  -d "provider=mock"
```

## Gateway Endpoints

- `GET /health` - Health check
- `POST /chat` - Chat with AI
  - Parameters:
    - `message` (required): The message to send
    - `provider` (optional): AI provider (deepseek, mock, simple, etc.)
    - `model` (optional): Model to use

## Troubleshooting

### Service won't start
```bash
# Check logs
sudo journalctl -u zen-claw -n 50

# Check if binary exists and is executable
ls -la /home/neves/git/zen-claw/zen-claw

# Check config file
cat ~/.zen/zen-claw/config.yaml
```

### Permission issues
```bash
# Make sure binary is executable
chmod +x /home/neves/git/zen-claw/zen-claw

# Check user in service file matches your username
# (Change "neves" to your username if different)
```

### Port already in use
The gateway runs on port 8080 by default. If something else is using it:
```bash
# Check what's using port 8080
sudo lsof -i :8080

# Or change the port in internal/gateway/gateway.go
# Look for: gw.server = &http.Server{Addr: ":8080", ...}
```

## Uninstalling

```bash
# Stop service
sudo systemctl stop zen-claw

# Disable on boot
sudo systemctl disable zen-claw

# Remove service file
sudo rm /etc/systemd/system/zen-claw.service

# Reload systemd
sudo systemctl daemon-reload
```

## Manual Testing (without systemd)

```bash
# Start gateway manually
cd ~/git/zen-claw
./zen-claw gateway start --config ~/.zen/zen-claw/config.yaml

# In another terminal, test it
curl http://localhost:8080/health

# Stop with Ctrl+C
```