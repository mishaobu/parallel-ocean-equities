.PHONY: test build run image

test:
	go test ./...
	npm --prefix web ci
	npm --prefix web run typecheck
	npm --prefix web test -- --run
	npm --prefix monetary ci
	npm --prefix monetary run typecheck
	npm --prefix monetary test -- --run

build:
	npm --prefix web ci
	npm --prefix web run build
	npm --prefix monetary ci
	npm --prefix monetary run build
	go build -o equities ./cmd/equities

run: build
	mkdir -p runtime
	DATA_FILE=runtime/state.json SEED_FILE=data/seed.json STATIC_DIR=web/dist MONETARY_STATIC_DIR=monetary/dist ./equities

image:
	docker build -t parallel-ocean-equities:local .
