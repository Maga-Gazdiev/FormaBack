.PHONY: run dev build test vet docker-up docker-down
run:
	docker compose up --build -d
dev:
	go run ./cmd/api
build:
	go build -trimpath -o bin/converter ./cmd/api
test:
	go test ./...
vet:
	go vet ./...
docker-up:
	docker compose up --build -d
docker-down:
	docker compose down
