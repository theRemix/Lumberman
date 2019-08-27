<p align="center">
  <img src="https://user-images.githubusercontent.com/132562/63731121-1730de80-c823-11e9-8eda-b8b44056944a.png" alt="Lumberman" />
</p>

<h1 align="center">Lumberman</h1>

<p align="center">
  <strong>Logger service using <a href="https://grpc.io">gRPC</a> stored in <a href="https://github.com/etcd-io/bbolt">bbolt (bolt db)</a></strong>
</p>


## Reference Clients

- [Lumberman-go-client](https://github.com/webmocha/Lumberman-go-client)
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

