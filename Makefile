.PHONY: build run test vet lint web-install web-build web-dev docker-build build-full

build:
	go build -o bin/certschedule ./cmd/server

# build-full builds the frontend, embeds it into internal/webui/dist, then
# builds the single self-contained backend binary that serves it.
build-full: web-build
	rm -rf internal/webui/dist
	mkdir -p internal/webui/dist
	cp -r web/dist/. internal/webui/dist/
	go build -o bin/certschedule ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

web-install:
	cd web && npm install

web-build:
	cd web && npm run build

web-dev:
	cd web && npm run dev

docker-build:
	docker build -t certschedule:latest .
