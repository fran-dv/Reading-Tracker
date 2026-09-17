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

# ---- Writing to the terminal -----------------------------------------
#
# The deploy speaks in the app's own ink: verdigris for what is done,
# rubric for what is wrong, pencil for the detail beside it. Colour is
# written only to a terminal that wants it, so a log or a pipe reads as
# plain text (NO_COLOR is honoured: https://no-color.org).

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	verdigris=$(printf '\033[38;5;79m')
	rubric=$(printf '\033[38;5;174m')
	pencil=$(printf '\033[38;5;247m')
	ink=$(printf '\033[38;5;255m')
	off=$(printf '\033[0m')
	hide=$(printf '\033[?25l')
	show=$(printf '\033[?25h')
	erase=$(printf '\r\033[2K')
	uline=$(printf '\033[4m')
	interactive=yes
else
	verdigris= rubric= pencil= ink= off= hide= show= erase= uline=
	interactive=no
fi

log=$(mktemp)
stty_saved=
cleanup() {
	rm -f "$log"
	if [ -n "$stty_saved" ]; then
		stty "$stty_saved"
	fi
	[ "$interactive" = yes ] && printf '%s' "$show"
	return 0
}
trap cleanup EXIT INT TERM

# One keypress, unechoed. An offer answered with a key should not leave that
# key sitting on the line the offer is written on, and Enter should go
# straight on. The terminal is put back however the script ends.
readkey() {
	key=
	if [ ! -t 0 ]; then
		read -r key || key=
		return 0
	fi
	stty_saved=$(stty -g)
	stty -icanon -echo min 1 time 0
	key=$(dd bs=1 count=1 2>/dev/null)
	stty "$stty_saved"
	stty_saved=
}

note() { printf '  %s%s%s\n' "$pencil" "$1" "$off"; }
done_() { printf '%s  %s✓%s  %-9s %s%s%s\n' "$erase" "$verdigris" "$off" "$1" "$pencil" "${2:-}" "$off"; }

fail() {
	printf '%s  %s✗%s  %-9s %s%s%s\n' "$erase" "$rubric" "$off" "$1" "$rubric" "${2:-}" "$off" >&2
	if [ -s "$log" ]; then
		sed 's/^/     /' "$log" >&2
	fi
	exit 1
}

# step LABEL DETAIL COMMAND... — runs the command with its output held
# back, and shows it only if the command fails. A terminal gets a spinner
# while it waits; anything else gets one line, because a spinner written
# to a file is just noise.
step() {
	label=$1
	detail=$2
	shift 2
	if [ "$interactive" = no ]; then
		"$@" >"$log" 2>&1 || fail "$label" "failed"
		done_ "$label" "$detail"
		return 0
	fi
	"$@" >"$log" 2>&1 &
	spinner $! "$label"
	wait $! 2>/dev/null || fail "$label" "failed"
	done_ "$label" "$detail"
}

# The spinner reads its pid and label from variables rather than taking
# arguments, because its own argument list is the frames it rotates through.
spinner() {
	spin_pid=$1
	spin_label=$2
	printf '%s' "$hide"
	set -- '⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏'
	while kill -0 "$spin_pid" 2>/dev/null; do
		printf '%s  %s%s%s  %s' "$erase" "$verdigris" "$1" "$off" "$spin_label"
		frame=$1
		shift
		set -- "$@" "$frame"
		sleep 0.08
	done
	printf '%s' "$show"
}

# ---- The deploy -------------------------------------------------------

printf '\n  %sReading Tracker%s %s· deploy%s\n\n' "$ink" "$off" "$pencil" "$off"

if [ -n "$(git status --porcelain)" ]; then
	fail "tree" "not clean; commit first"
fi

commit=$(git rev-parse --short HEAD)
step "tests" "every package" go test ./...
step "install" "$commit" go install ./cmd/readingqueue

binary="$HOME/go/bin/readingqueue"
config="$HOME/.config/readingqueue"
address="$config/address"

# Port 80 needs the capability granted again after every install. The
# answer is remembered in the address file, so the question is asked once;
# with no terminal to ask in, the app stays on 8080.
port=8080
if [ -f "$address" ] && grep -q ':80$' "$address"; then
	port=80
elif [ "$interactive" = yes ]; then
	printf '\n  %sthe port%s\n' "$ink" "$off"
	note "Open the app at http://readingtracker.localhost, with no port?"
	note "Declined, it stays at http://readingtracker.localhost:8080."
	note "Your answer is remembered, so this is asked once."
	printf '\n  %s[y/N]%s ' "$ink" "$off"
	read -r reply
	printf '\n'
	case "$reply" in
	[yY] | [yY][eE][sS]) port=80 ;;
	esac
fi

if [ "$port" = 80 ]; then
	# The password prompt is the one moment the deploy asks for something it
	# cannot explain itself, so the command is written out first, and sudo is
	# given a prompt in the same voice instead of its bare default. A
	# password already cached needs no preamble, so none is printed.
	asked=no
	if ! sudo -n true 2>/dev/null; then
		asked=yes
		printf '\n  %ssudo is required to run the app at %s%s%s\n' \
			"$pencil" "$verdigris" "http://readingtracker.localhost" "$off"
		note "Nothing else is run as root."
		# Nothing to press when there is no terminal: the offer would only
		# swallow a line of input meant for sudo itself.
		if [ "$interactive" = yes ]; then
			tip="    ${uline}${ink}?${off}${pencil} to see the command${off}"
			while :; do
				printf '\n  %s[Enter]%s%s to continue%s%s' \
					"$ink" "$off" "$pencil" "$off" "$tip"
				readkey
				case "$key" in
				'?')
					printf '\n\n  %ssetcap cap_net_bind_service=+ep %s%s\n' "$verdigris" "$binary" "$off"
					tip= # asked and answered; offering it again is noise
					;;
				*) break ;;
				esac
			done
		fi
		printf '\n\n'
	fi
	# %p is sudo's own placeholder for whose password is wanted; the shell
	# must not touch it, so the prompt is built by expansion and never
	# through printf.
	ask="  ${ink}password for %p${off} "
	if sudo -p "$ask" setcap 'cap_net_bind_service=+ep' "$binary"; then
		# The blank line parts the prompt from the tick; with the password
		# already cached there was no prompt to part from.
		if [ "$asked" = yes ]; then
			printf '\n'
		fi
		done_ "port 80" "granted"
	else
		printf '%s  %s!%s  %-9s %snot granted; staying on 8080%s\n' \
			"$erase" "$rubric" "$off" "port 80" "$pencil" "$off" >&2
		port=8080
	fi
fi

mkdir -p "$config"
echo "READINGQUEUE_ADDR=127.0.0.1:$port" >"$address"

step "service" "installed and reloaded" sh -c '
	install -Dm644 deploy/readingqueue.service ~/.config/systemd/user/readingqueue.service
	systemctl --user daemon-reload
	systemctl --user stop readingqueue
'

data="${XDG_DATA_HOME:-$HOME/.local/share}/readingqueue"
if [ -f "$data/readingqueue.db" ]; then
	mkdir -p "$data/backups"
	cp "$data/readingqueue.db" "$data/backups/pre-deploy-$commit.db"
	done_ "database" "copied to backups/pre-deploy-$commit.db"
else
	done_ "database" "none yet; it is created on first start"
fi

step "running" "$commit" systemctl --user enable --now readingqueue

url="http://readingtracker.localhost"
[ "$port" = 80 ] || url="$url:$port"

# The last thing on the screen is the one thing to do next.
printf '\n  %sThe app is running and ready at%s\n' "$pencil" "$off"
printf '  %s%s%s\n' "$verdigris" "$url" "$off"
printf '  %sIt starts again by itself when you log in.%s\n\n' "$pencil" "$off"
