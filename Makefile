build:
	go build -o indexer cmd/indexer/main.go

run: build
	./indexer