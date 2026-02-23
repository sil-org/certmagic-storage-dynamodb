test: deps
	docker compose run test

deps:
	go get ./...

.PHONY: test deps
