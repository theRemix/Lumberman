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
)

const (
	logsBucket     = "Logs"
	prefixesBucket = "Prefixes"
)

var (
	dbPath = flag.String("db_file", "lumberman.db", "Path to DB file")
	port   = flag.Int("port", 9090, "The server port")
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

	grpcServer := grpc.NewServer()
	pb.RegisterLoggerServer(grpcServer, NewLogServer(db))
	grpcServer.Serve(lis)
}
