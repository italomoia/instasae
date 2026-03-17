#!/bin/bash
# Deploy instasae to Docker Swarm
# Usage: ./deploy.sh

set -e

echo "Building Docker image..."
docker build -t ghcr.io/italomoia/instasae:latest .

echo "Pushing to GHCR..."
docker push ghcr.io/italomoia/instasae:latest

echo "Done! To deploy on the VPS, run:"
echo "  docker pull ghcr.io/italomoia/instasae:latest"
echo "  docker stack deploy -c docker-compose.prod.yml instasae"
echo ""
echo "To update an existing deployment:"
echo "  docker service update --image ghcr.io/italomoia/instasae:latest instasae_instasae --force"
