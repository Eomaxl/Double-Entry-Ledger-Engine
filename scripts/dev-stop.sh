#!/bin/bash

# Development Environment Stop Script
# Stops Docker Compose services for the Double-Entry Ledger Engine.

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

if ! docker info &> /dev/null; then
    print_error "Docker is not running. Please start Docker and try again."
    exit 1
fi

COMPOSE_CMD=""
if docker compose version &> /dev/null; then
    COMPOSE_CMD="docker compose"
elif command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker-compose"
else
    print_error "Neither 'docker compose' nor 'docker-compose' is available."
    exit 1
fi

print_header "Stopping Double-Entry Ledger Engine Development Environment"

if [ "$1" = "--volumes" ] || [ "$1" = "-v" ]; then
    print_warning "Removing volumes and orphans as well..."
    $COMPOSE_CMD down -v --remove-orphans
else
    $COMPOSE_CMD down --remove-orphans
fi

print_status "Services stopped successfully."
