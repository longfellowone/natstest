#! /bin/bash
cd cmd/gossip
GOOS=linux CGO_ENABLED=0 go build -o gossip .
echo "Starting services ..."
docker-compose up --build