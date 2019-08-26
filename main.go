package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	pb "github.com/webmocha/lumberman/pb"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/testdata"
)

const (
	logsBucket     = "Logs"
	prefixesBucket = "Prefixes"
)

var (
	tls      = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile = flag.String("cert_file", "", "The TLS cert file")
	keyFile  = flag.String("key_file", "", "The TLS key file")
	dbPath   = flag.String("db_file", "lumberman.db", "Path to DB file")
	port     = flag.Int("port", 9090, "The server port")
)

func main() {
	flag.Parse()

	db, err := bolt.Open(*dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		var err error
		_, err = tx.CreateBucketIfNotExists([]byte(logsBucket))
		if err != nil {
			return fmt.Errorf("Error creating bucket %s: %s", logsBucket, err)
		}
		return nil
	})

	db.Update(func(tx *bolt.Tx) error {
		var err error
		_, err = tx.CreateBucketIfNotExists([]byte(prefixesBucket))
		if err != nil {
			return fmt.Errorf("Error creating bucket %s: %s", prefixesBucket, err)
		}
		return nil
	})

	log.Printf("Loaded DB %s", *dbPath)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("Server listening on port %d", *port)

	var opts []grpc.ServerOption
	if *tls {
		if *certFile == "" {
			*certFile = testdata.Path("server1.pem")
		}
		if *keyFile == "" {
			*keyFile = testdata.Path("server1.key")
		}
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterLoggerServer(grpcServer, NewLogServer(db))
	grpcServer.Serve(lis)
}
