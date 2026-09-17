#!/bin/sh
# Installs the current commit as the app you use day to day (http://127.0.0.1:8080).
#
# Migrations only run forward, so the database is copied before the new
# binary first opens it. To roll back: stop the service, restore that copy,
# and deploy the older commit.
set -eu
cd "$(dirname "$0")/.."

if [ -n "$(git status --porcelain)" ]; then
	echo "deploy: working tree is not clean; commit first" >&2
	exit 1
fi

go test ./...
go install ./cmd/readingqueue

install -Dm644 deploy/readingqueue.service ~/.config/systemd/user/readingqueue.service
systemctl --user daemon-reload
systemctl --user stop readingqueue

data="${XDG_DATA_HOME:-$HOME/.local/share}/readingqueue"
if [ -f "$data/readingqueue.db" ]; then
	mkdir -p "$data/backups"
	cp "$data/readingqueue.db" "$data/backups/pre-deploy-$(git rev-parse --short HEAD).db"
fi

systemctl --user enable --now readingqueue
echo "deployed $(git rev-parse --short HEAD) to http://127.0.0.1:8080"
