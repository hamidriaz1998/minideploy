BINARY=minideploy
BUILD_DIR=build

.PHONY: all build clean cross install

all: build

build:
	go build -o $(BUILD_DIR)/$(BINARY) .

cross:
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY)-linux-arm64 .

clean:
	rm -rf $(BUILD_DIR)

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

tidy:
	go mod tidy

test:
	go test ./...
