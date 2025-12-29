build:
	go build -o bin/indexer cmd/main.go

run: build
	@echo "🧹 Running..."
	./bin/indexer

clean:
	@echo "🧹 Cleaning..."
	rm -f indexer
	@echo "✅ Clean complete!"

test:
	@echo "🧪 Running tests..."
	go test ./...