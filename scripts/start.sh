#!/bin/bash

# Double-Entry Ledger Engine Startup Script
# This script starts the application with proper environment setup

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# Check if .env file exists
if [ ! -f .env ]; then
    print_warning ".env file not found, copying from .env.example"
    if [ -f .env.example ]; then
        cp .env.example .env
        print_status "Created .env file from .env.example"
        print_warning "Please review and update the configuration in .env file"
    else
        print_error ".env.example file not found"
        exit 1
    fi
fi

# Load environment variables
if [ -f .env ]; then
    print_status "Loading environment variables from .env"
    set -a
    # shellcheck disable=SC1091
    . ./.env
    set +a
fi

# Check required environment variables
required_vars=("DB_HOST" "DB_USER" "DB_NAME" "NATS_URL")
missing_vars=()

for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        missing_vars+=("$var")
    fi
done

if [ ${#missing_vars[@]} -ne 0 ]; then
    print_error "Missing required environment variables:"
    for var in "${missing_vars[@]}"; do
        echo "  - $var"
    done
    exit 1
fi

# Check if binary exists
if [ ! -f "./server" ]; then
    print_status "Building application..."
    go build -o server ./cmd/server
    if [ $? -ne 0 ]; then
        print_error "Failed to build application"
        exit 1
    fi
    print_status "Application built successfully"
fi

# Check database connectivity
print_status "Checking database connectivity..."
if command -v psql &> /dev/null; then
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" &> /dev/null
    if [ $? -eq 0 ]; then
        print_status "Database connection successful"
    else
        print_error "Cannot connect to database. Please check your database configuration."
        exit 1
    fi
else
    print_warning "psql not found, skipping database connectivity check"
fi

# Check NATS connectivity (if nats CLI is available)
if command -v nats &> /dev/null; then
    print_status "Checking NATS connectivity..."
    nats server check --server="$NATS_URL" &> /dev/null
    if [ $? -eq 0 ]; then
        print_status "NATS connection successful"
    else
        print_warning "Cannot connect to NATS server. The application will attempt to connect on startup."
    fi
else
    print_warning "nats CLI not found, skipping NATS connectivity check"
fi

# Start the application
print_status "Starting Double-Entry Ledger Engine..."
print_status "Server will be available at http://${SERVER_HOST:-0.0.0.0}:${SERVER_PORT:-8080}"
print_status "Health check: http://${SERVER_HOST:-0.0.0.0}:${SERVER_PORT:-8080}/health"
print_status "Metrics: http://${SERVER_HOST:-0.0.0.0}:${SERVER_PORT:-8080}/metrics"

exec ./server