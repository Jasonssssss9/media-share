SHELL := /bin/bash

.PHONY: help install services stop dev-api dev-web fmt test test-db migrate-up migrate-down migrate-status lint build check

help:
	@printf '%s\n' 'make install   Install project dependencies' 'make services  Start PostgreSQL and Elasticsearch' 'make dev-api   Run the Go API' 'make dev-web   Run the React development server' 'make migrate-up  Apply database migrations' 'make migrate-down  Roll back the latest migration' 'make migrate-status  Show migration status' 'make test-db   Test migrations and authentication' 'make check     Run tests, lint, and builds' 'make stop      Stop supporting services'

install:
	cd backend && go mod download
	cd web && npm ci

services:
	docker compose up -d --wait --wait-timeout 240

stop:
	docker compose stop

dev-api:
	@set -eu; set -a; source .env; set +a; cd backend; go run ./cmd/api

dev-web:
	@set -eu; set -a; source .env; set +a; cd web; npm run dev

fmt:
	gofmt -w backend

test:
	cd backend && go test ./...
	cd web && npm test

test-db:
	@set -eu; set -a; source .env; set +a; cd backend; go test -tags=integration ./integration -count=1

migrate-up:
	@set -eu; set -a; source .env; set +a; cd backend; go run ./cmd/migrate up

migrate-down:
	@set -eu; set -a; source .env; set +a; cd backend; go run ./cmd/migrate down

migrate-status:
	@set -eu; set -a; source .env; set +a; cd backend; go run ./cmd/migrate status

lint:
	cd backend && go vet ./...
	cd web && npm run lint

build:
	cd backend && go build -o ../var/bin/api ./cmd/api
	cd web && npm run build

check: test lint build
