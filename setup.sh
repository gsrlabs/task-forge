#!/usr/bin/env bash

set -e

REPO_URL="https://github.com/gsrlabs/task-forge.git"
PROJECT_DIR="task-forge"

echo "🚀 Installing Task Forge..."

# Check dependencies
if ! command -v git >/dev/null 2>&1; then
    echo "❌ Git is not installed."
    exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "❌ Docker is not installed."
    exit 1
fi

# Clone repository
if [ -d "$PROJECT_DIR" ]; then
    echo "📁 Directory '$PROJECT_DIR' already exists."
    cd "$PROJECT_DIR"

    echo "🔄 Updating repository..."
    git pull
else
    echo "📥 Cloning repository..."
    git clone "$REPO_URL"
    cd "$PROJECT_DIR"
fi

# Create .env
if [ ! -f ".env" ]; then
    echo "⚙️ Creating .env from .env.example..."
    cp .env.example .env
else
    echo "ℹ️ .env already exists, keeping it."
fi

# Start application
echo "🐳 Starting Task Forge..."
docker compose up --build