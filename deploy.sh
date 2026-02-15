#!/bin/bash

set -e

REPO="git@github.com:rainett/bandit-counter-bot.git"
DIR="$(cd "$(dirname "$0")" && pwd)"
DB_FILE="$DIR/slotbot.db"
BACKUP_DIR="$DIR/backups"

# Створюємо папку для бекапів
mkdir -p "$BACKUP_DIR"

# Поточна дата для імені бекапу
DATE=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="$BACKUP_DIR/slotbot_$DATE.db"

if [ -f "$DB_FILE" ]; then
    echo "Backing up database to $BACKUP_FILE..."
    cp "$DB_FILE" "$BACKUP_FILE"
fi

echo "Updating repository..."
if [ -d "$DIR/.git" ]; then
  cd "$DIR"
  git fetch --all
  git reset --hard origin/main
else
  git clone "$REPO" "$DIR"
  cd "$DIR"
fi

echo "Building bot..."
go build -o slotbot ./cmd/bot

echo "Killing old process..."
pkill -f slotbot || true

echo "Starting new bot..."
nohup ./slotbot > slotbot.log 2>&1 &

echo "Deployed!"