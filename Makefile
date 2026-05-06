.PHONY: dev api web build test tidy migrate reset-admin-password migrate-help

dev:
	@echo "Run 'make api' and 'make web' in separate terminals."

api:
	go run ./cmd/fluffcatch

migrate:
	go run ./cmd/fluffcatch --migrate

reset-admin-password:
	go run ./cmd/fluffcatch --reset-admin-password

web:
	cd www && npm install && npm run dev

build:
	cd www && npm install && npm run build
	go build -tags embed_frontend -o bin/fluffcatch ./cmd/fluffcatch

test:
	go test ./...
	cd www && npm install && npm run build

tidy:
	go mod tidy

migrate-help:
	@echo "Apply migrations with your preferred MySQL tool, e.g.:"
	@echo "mysql -u fluffcatch -p fluffcatch < migrations/001_initial_schema.sql"
