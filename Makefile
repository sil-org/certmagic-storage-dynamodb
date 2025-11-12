test: deps
	docker compose run test

deps:
	go get ./...
