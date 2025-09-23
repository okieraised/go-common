.PHONY: lint test test-cover update-all

lint:
	golangci-lint run ./...

update-all:
	go get -u ./...

test:
	go test -coverprofile=c.out -coverpkg=./... ./...
	go tool cover -html=c.out -o test-coverage.html