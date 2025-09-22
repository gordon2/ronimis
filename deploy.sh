#!/bin/bash

# Check if required variables are set
if [ -z "$SERVER_IP" ] || [ -z "$SERVER_USER" ]; then
    echo "Error: Required environment variables not set"
    echo "Usage: SERVER_IP=your.server.ip SERVER_USER=username ./deploy.sh"
    exit 1
fi

echo "Deploying to ${SERVER_USER}@${SERVER_IP}..."

# Upload application files
echo "Uploading application files..."
scp server.go gym-stats-collector.sh dashboard.html ${SERVER_USER}@${SERVER_IP}:/home/${SERVER_USER}/ronimis/

# Upload service files
echo "Uploading service files..."
scp services/*.service ${SERVER_USER}@${SERVER_IP}:/tmp/

# Install services and restart
echo "Installing services and restarting..."
ssh -t ${SERVER_USER}@${SERVER_IP} "
    sudo mv /tmp/*.service /etc/systemd/system/ && \
    sudo systemctl daemon-reload && \
    sudo systemctl restart gym.service gym-stats-collector.service && \
    echo 'Deployment complete!' && \
    sudo systemctl status gym.service --no-pager -l
"