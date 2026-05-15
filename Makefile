BREW_PREFIX  ?= $(shell brew --prefix)
DATABASE_URL ?= "postgres://$(USER)@localhost/apollo_test?sslmode=disable"

test:
	@DATABASE_URL=$(DATABASE_URL) go test -race -timeout 1s ./...

test-setup: $(BREW_PREFIX)/bin/migrate
	migrate -path migrations/ -database $(DATABASE_URL) up

build:
	@go build ./cmd/apollo

lint:
	@golangci-lint run

$(BREW_PREFIX)/bin/migrate:
	@brew install golang-migrate

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f --tail=100

docker-migrate:
	docker compose run --rm migrate

docker-psql:
	docker compose exec postgres psql -U apollo apollo

docker-nuke:
	docker compose down -v

.PHONY: all build deps lint test docker-build docker-up docker-down docker-logs docker-migrate docker-psql docker-nuke
