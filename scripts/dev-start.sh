#!/bin/bash

# Development Environment Startup Script
# This script starts the complete development environment using Docker Compose

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}[DEV]${NC} $1"
}

# Check if Docker is running
if ! docker info &> /dev/null; then
    print_error "Docker is not running. Please start Docker and try again."
    exit 1
fi

# Resolve compose command (supports both plugin and standalone)
COMPOSE_CMD=""
if docker compose version &> /dev/null; then
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    print_error "Neither 'docker compose' nor 'docker-compose' is available."
    exit 1
fi

print_header "Starting Double-Entry Ledger Engine Development Environment"

# Check if .env file exists
if [ ! -f .env ]; then
    print_warning ".env file not found, copying from .env.example"
    if [ -f .env.example ]; then
        cp .env.example .env
        print_status "Created .env file from .env.example"
    else
        print_error ".env.example file not found"
        exit 1
    fi
fi

# Stop any existing containers
print_status "Stopping existing containers..."
$COMPOSE_CMD down --remove-orphans

# Build and start services
print_status "Building and starting services..."
$COMPOSE_CMD up --build -d

# Wait for services to be healthy
print_status "Waiting for services to be ready..."

# Wait for PostgreSQL
print_status "Waiting for PostgreSQL..."
timeout=60
counter=0
while ! $COMPOSE_CMD exec -T postgres pg_isready -U postgres &> /dev/null; do
    if [ $counter -ge $timeout ]; then
        print_error "PostgreSQL failed to start within $timeout seconds"
        $COMPOSE_CMD logs postgres
        exit 1
    fi
    sleep 1
    counter=$((counter + 1))
done
print_status "PostgreSQL is ready"

# Wait for NATS
print_status "Waiting for NATS..."
counter=0
while ! curl -s http://localhost:8222/healthz &> /dev/null; do
    if [ $counter -ge $timeout ]; then
        print_error "NATS failed to start within $timeout seconds"
        $COMPOSE_CMD logs nats
        exit 1
    fi
    sleep 1
    counter=$((counter + 1))
done
print_status "NATS is ready"

# Wait for Ledger Engine
print_status "Waiting for Ledger Engine..."
counter=0
while ! curl -s http://localhost:8080/health &> /dev/null; do
    if [ $counter -ge $timeout ]; then
        print_error "Ledger Engine failed to start within $timeout seconds"
        $COMPOSE_CMD logs ledger-engine
        exit 1
    fi
    sleep 1
    counter=$((counter + 1))
done
print_status "Ledger Engine is ready"

# Display service information
print_header "Development Environment Ready!"
echo ""
echo "Services:"
echo "  📊 Ledger Engine API: http://localhost:8080"
echo "  🏥 Health Check:      http://localhost:8080/health"
echo "  📈 Metrics:           http://localhost:8080/metrics"
echo "  🗄️  PostgreSQL:        localhost:5432"
echo "  📡 NATS:              localhost:4222"
echo "  📊 NATS Monitoring:   http://localhost:8222"
echo ""
echo "API Documentation:"
echo "  📚 Swagger UI:        http://localhost:8080/swagger/index.html (if enabled)"
echo ""
echo "Default API Key: dev-key-12345"
echo ""
echo "Example API calls:"
echo "  # Create account"
echo "  curl -X POST http://localhost:8080/v1/accounts \\"
echo "    -H 'X-API-Key: dev-key-12345' \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"account_type\":\"asset\",\"currencies\":[{\"currency_code\":\"USD\",\"allow_negative\":false}]}'"
echo ""
echo "  # Check health"
echo "  curl http://localhost:8080/health"
echo ""
echo "Useful commands:"
echo "  📋 View logs:         $COMPOSE_CMD logs -f"
echo "  🔍 View app logs:     $COMPOSE_CMD logs -f ledger-engine"
echo "  🛑 Stop services:     $COMPOSE_CMD down"
echo "  🔄 Restart services:  $COMPOSE_CMD restart"
echo "  🧹 Clean up:         $COMPOSE_CMD down -v --remove-orphans"

# Optionally show logs
if [ "$1" = "--logs" ] || [ "$1" = "-l" ]; then
    echo ""
    print_status "Showing application logs (Ctrl+C to exit)..."
    $COMPOSE_CMD logs -f ledger-engine
fi