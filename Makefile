.PHONY: build test vet up down seed migrate-up migrate-version clean

build:
	go build -o bin/api ./cmd/api
	go build -o bin/migrate ./cmd/migrate
	go build -o bin/seed ./cmd/seed

test:
	go test -v ./...
	go vet ./...

vet:
	go vet ./...

up:
	docker compose up --build

down:
	docker compose down

seed:
	docker compose run --rm seed

migrate-up:
	docker compose run --rm migrate up

migrate-version:
	docker compose run --rm migrate version

clean:
	rm -rf bin/
