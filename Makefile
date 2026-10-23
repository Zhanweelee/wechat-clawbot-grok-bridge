.PHONY: build test install

BINARY=wechat-clawbot-grok-bridge

build:
	go build -o bin/$(BINARY) ./cmd/wechat-clawbot-grok-bridge

test:
	go test ./...

install:
	go install ./cmd/wechat-clawbot-grok-bridge
