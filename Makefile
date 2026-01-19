.PHONY: build run test clean install

build:
	go build -o jj-diff ./cmd/jj-diff

run: build
	./jj-diff

test:
	go test -v ./...

clean:
	rm -f jj-diff

install: build
	go install ./cmd/jj-diff

deps:
	go mod download
	go mod tidy
