build:
	go build -o bin/lumberman .

build-linux64:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/lumberman .

proto:
	protoc --go_out=plugins=grpc:./pb *.proto

run:
	./bin/lumberman

dev: build run

dev-insecure: build run

.PHONY: certs build build-linux64 proto run dev dev-insecure
