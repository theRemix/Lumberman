certPath = ./certs
country = US
state = Washington
locality = Seattle
organization = Test

certs:
	openssl genrsa -passout pass:1111 -des3 -out $(certPath)/ca.key 4096
	openssl req -passin pass:1111 -new -x509 -days 365 -key $(certPath)/ca.key -out $(certPath)/ca.crt -subj  "/C=$(country)/ST=$(state)/L=$(locality)/O=$(organization)/OU=DevCA/CN=ca"
	openssl genrsa -passout pass:1111 -des3 -out $(certPath)/server.key 4096
	openssl req -passin pass:1111 -new -key $(certPath)/server.key -out $(certPath)/server.csr -subj "/C=$(country)/ST=$(state)/L=$(locality)/O=$(organization)/OU=DevServer/CN=localhost"
	openssl x509 -req -passin pass:1111 -days 365 -in $(certPath)/server.csr -CA $(certPath)/ca.crt -CAkey $(certPath)/ca.key -set_serial 01 -out $(certPath)/server.crt
	openssl rsa -passin pass:1111 -in $(certPath)/server.key -out $(certPath)/server.key
	openssl genrsa -passout pass:1111 -des3 -out $(certPath)/client.key 4096
	openssl req -passin pass:1111 -new -key $(certPath)/client.key -out $(certPath)/client.csr -subj "/C=$(country)/ST=$(state)/L=$(locality)/O=$(organization)/OU=DevClient/CN=localhost"
	openssl x509 -passin pass:1111 -req -days 365 -in $(certPath)/client.csr -CA $(certPath)/ca.crt -CAkey $(certPath)/ca.key -set_serial 01 -out $(certPath)/client.crt
	openssl rsa -passin pass:1111 -in $(certPath)/client.key -out $(certPath)/client.key

build:
	go build -o bin/lumberman .

build-linux64:
	env GOOS=linux GOARCH=amd64 go build -o bin/lumberman .

proto:
	protoc --go_out=plugins=grpc:./pb *.proto

run-tls:
	./bin/lumberman -tls true -certFile certs/server.crt -keyFile certs/server.key

run-insecure:
	./bin/lumberman

dev: build run-tls

dev-insecure: build run-insecure

.PHONY: certs build build-linux64 proto run-tls run-insecure dev dev-insecure
