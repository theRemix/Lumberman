# Lumberman

Logger service using [gRPC](https://grpc.io/) saving to [bbolt (bolt db)](https://github.com/etcd-io/bbolt)

## Reference Clients

- [Lumberman-node-client](https://github.com/webmocha/Lumberman-node-client)


## Install and Usage

```sh
go get github.com/webmocha/Lumberman
```

Run with defaults
```sh
Lumberman
```

specify a db file path

```sh
Lumberman -db_file /var/db/lumberman.db
```

## Service Definition

see [lumber.proto](./lumber.proto)

## Dev

generate certs

```sh
make certs
```

updating .proto files

```
make proto
```

running project

```sh
ag -g '\.go' . | entr sh -c 'clear && make dev'
```

