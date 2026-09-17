#!/bin/sh
# Installs the current commit as the app you use day to day, at
# http://readingtracker.localhost (or :8080; see "the port" below).
#
# The name costs nothing: .localhost is reserved for the loopback address
# (RFC 6761), so any name under it already resolves here, with no hosts
# file, no daemon and no configuration.
#
# The port does cost something. A bare URL means port 80, and Linux keeps
# ports under 1024 for root, so the binary has to be granted the one
# capability that lets it bind low ports. This script asks before doing
# that, once, and takes no for an answer: declined, the app runs on 8080
# and the name still works, with the port typed after it. `go install`
# replaces the binary on every deploy and capabilities live on the file,
# so the grant is re-applied each time you deploy.
#
# To go back to 8080: rm ~/.config/readingqueue/address, then deploy.
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

binary="$HOME/go/bin/readingqueue"
config="$HOME/.config/readingqueue"
address="$config/address"

# Port 80 needs the capability granted again after every install. The
# answer is remembered in the address file, so the question is asked once;
# with no terminal to ask in, the app stays on 8080.
port=8080
if [ -f "$address" ] && grep -q ':80$' "$address"; then
	port=80
elif [ -t 0 ]; then
	printf 'Open the app at http://readingtracker.localhost, with no port?\n'
	printf 'That needs sudo once per deploy, to let it hold port 80. [y/N] '
	read -r reply
	case "$reply" in
	[yY] | [yY][eE][sS]) port=80 ;;
	esac
fi

if [ "$port" = 80 ] && ! sudo setcap 'cap_net_bind_service=+ep' "$binary"; then
	echo "deploy: could not grant port 80; staying on 8080" >&2
	port=8080
fi

mkdir -p "$config"
echo "READINGQUEUE_ADDR=127.0.0.1:$port" >"$address"

install -Dm644 deploy/readingqueue.service ~/.config/systemd/user/readingqueue.service
systemctl --user daemon-reload
systemctl --user stop readingqueue

data="${XDG_DATA_HOME:-$HOME/.local/share}/readingqueue"
if [ -f "$data/readingqueue.db" ]; then
	mkdir -p "$data/backups"
	cp "$data/readingqueue.db" "$data/backups/pre-deploy-$(git rev-parse --short HEAD).db"
fi

systemctl --user enable --now readingqueue

url="http://readingtracker.localhost"
[ "$port" = 80 ] || url="$url:$port"
echo "deployed $(git rev-parse --short HEAD) to $url"
