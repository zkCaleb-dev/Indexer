.PHONY: help db-up db-down db-reset db-logs build run clean test pgadmin

help:
	@echo "Available commands:"
	@echo "  make db-up       - Start PostgreSQL container"
	@echo "  make db-down     - Stop PostgreSQL container"
	@echo "  make db-reset    - Reset database (stop, remove volumes, start)"
	@echo "  make db-logs     - Show PostgreSQL logs"
	@echo "  make build       - Build the indexer binary"
	@echo "  make run         - Run the indexer (ensure db-up first)"
	@echo "  make clean       - Clean built binaries"
	@echo "  make test        - Run tests"
	@echo "  make pgadmin     - Start PostgreSQL + pgAdmin"

db-up:
	@echo "🚀 Starting PostgreSQL..."
	docker compose up -d postgres
	@echo "⏳ Waiting for PostgreSQL to be ready..."
	@sleep 3
	@docker compose exec -T postgres pg_isready -U indexer || (echo "❌ PostgreSQL not ready" && exit 1)
	@echo "✅ PostgreSQL is ready!"

db-down:
	@echo "🛑 Stopping PostgreSQL..."
	docker compose down

db-reset:
	@echo "🔄 Resetting database..."
	docker compose down -v
	@echo "✅ Database reset complete"
	@make db-up

db-logs:
	docker compose logs -f postgres

pgadmin:
	@echo "🚀 Starting PostgreSQL + pgAdmin..."
	docker compose --profile tools up -d
	@echo "✅ PostgreSQL: localhost:5433"
	@echo "✅ pgAdmin: http://localhost:5050 (admin@indexer.local / admin)"

build:
	go build -o bin/indexer cmd/main.go

run: build
	./bin/indexer

clean:
	@echo "🧹 Cleaning..."
	rm -f indexer
	@echo "✅ Clean complete!"

test:
	@echo "🧪 Running tests..."
	go test ./...