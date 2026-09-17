# Everyday commands. Development stays on its own port and its own database,
# so the app you actually read with is never the one being changed.
#
# Templates, styles and fonts are embedded in the binary, so `make dev` has to
# be restarted to see a change to any of them. Only the database is live.

DEV_DB   ?= .dev/readingqueue.db
DEV_PORT ?= 8081

.PHONY: help dev lan test check deploy

help:
	@echo 'make dev      the app at http://127.0.0.1:$(DEV_PORT), on $(DEV_DB)'
	@echo 'make lan      the same, reachable from the phone on this network'
	@echo 'make test     go test ./...'
	@echo 'make check    formatting, vet and tests: run before committing'
	@echo 'make deploy   install the current commit as the app you use'

dev:
	go run ./cmd/readingqueue -addr 127.0.0.1:$(DEV_PORT) -db $(DEV_DB)

# Every interface, so a phone can reach it. The address to open is printed
# because it is this machine's on the network, not localhost: asking the
# routing table for the way out gives the address the phone can see.
lan:
	@ip="$$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{print $$7; exit}')"; \
	if [ -n "$$ip" ]; then echo "on this network: http://$$ip:$(DEV_PORT)"; \
	else echo "on this network: port $(DEV_PORT), at this machine's address"; fi
	go run ./cmd/readingqueue -addr 0.0.0.0:$(DEV_PORT) -db $(DEV_DB)

test:
	go test ./...

# gofmt reports what it would change and says nothing when there is nothing
# to change, so anything printed is a failure.
check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt:"; echo "$$unformatted"; exit 1; fi
	go vet ./...
	go test ./...

deploy:
	./deploy/deploy.sh
