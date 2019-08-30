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

## Options

| flag | default | description |
| ---- | ------- | ----------- |
| -db_file | lumberman.db | Path to DB file |
| -port | 9090 | Port to listen for connections |

## Install and run with Go

```sh
go get github.com/webmocha/Lumberman
```

Run with defaults
```sh
Lumberman
```

specify a db file path and port

```sh
Lumberman -db_file /var/db/lumberman.db -port 12345
```

## Run with Docker :whale:

```sh
docker run -d \
  --name lumberman
  -p 9090:9090 \
  quay.io/theremix/lumberman
```

Persist db on host fs

```sh
docker run -d \
  --name lumberman
  -p 9090:9090 \
  -v /var/db/lumberman/:/data/
  quay.io/theremix/lumberman -dbPath /data/lumberman.db
```

## Service Definition

see [lumber.proto](./lumber.proto)


## Dev

updating .proto files

```
make proto
```

running project

```sh
ag -g '\.go' . | entr sh -c 'clear && make dev'
```

