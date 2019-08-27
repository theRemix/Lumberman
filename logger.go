package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"log"

	timestamp "github.com/golang/protobuf/ptypes"
	empty "github.com/golang/protobuf/ptypes/empty"
	pb "github.com/webmocha/lumberman/pb"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogServer struct {
	db      *bolt.DB
	streams map[string](chan *pb.GetLogReply)
}

func NewLogServer(db *bolt.DB) *LogServer {
	return &LogServer{
		db:      db,
		streams: make(map[string](chan *pb.GetLogReply)),
	}
}

func (s *LogServer) storePrefix(prefix string) {

	if dbErr := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(prefixesBucket))
		key := []byte(prefix)
		v := b.Get(key)
		if v != nil {
			return nil
		}
		return b.Put(key, nil)
	}); dbErr != nil {
		log.Printf("[storePrefix()] Error storing prefix (%s): %v\n", prefix, dbErr)
	}
}

func (s *LogServer) broadcastToStreams(prefix string, logReply *pb.GetLogReply) {
	if stream, ok := s.streams[prefix]; ok {
		stream <- logReply
	}
}

func (s *LogServer) getStreamChan(prefix string) chan *pb.GetLogReply {
	if stream, ok := s.streams[prefix]; ok {
		return stream
	}
	stream := make(chan *pb.GetLogReply)
	s.streams[prefix] = stream
	return stream
}

// Write to Log
func (s *LogServer) Log(ctx context.Context, req *pb.LogRequest) (*pb.LogReply, error) {

	logReply := &pb.GetLogReply{
		Data:      req.GetData(),
		Timestamp: timestamp.TimestampNow(),
	}
	logReply.Key = req.GetPrefix() + "|" + timestamp.TimestampString(logReply.GetTimestamp())

	logBytes, encodeLogErr := encodeLog(logReply)

	if encodeLogErr != nil {
		log.Printf("[Log()] Error encoding GetLogReply with gob: %+v\n", encodeLogErr)
		return nil, status.Error(codes.Internal, "Error encoding GetLogReply")
	}

	if dbErr := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(logsBucket))
		key := []byte(logReply.GetKey())
		err := b.Put(key, logBytes)
		return err
	}); dbErr != nil {
		log.Printf("[Log()] Error saving to db: %+v\n", dbErr)
		return nil, status.Errorf(codes.Internal, "Error saving to db (key: %s)", logReply.GetKey())
	}

	go s.broadcastToStreams(req.GetPrefix(), logReply)

	go s.storePrefix(req.GetPrefix())

	return &pb.LogReply{
		Key: logReply.GetKey(),
	}, nil
}

// Get Log by key
func (s *LogServer) GetLog(ctx context.Context, req *pb.GetLogRequest) (*pb.GetLogReply, error) {
	key := []byte(req.GetKey())
	var logReplyBytes []byte

	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(logsBucket))
		logReplyBytes = b.Get(key)
		return nil
	}); err != nil {
		log.Printf("[GetLog()] Error reading from db (key: %s): %v\n", key, err)
		return nil, status.Errorf(codes.Internal, "Error reading from db (key: %s)", key)
	}

	logReply, err := decodeLog(*bytes.NewBuffer(logReplyBytes))
	if err != nil {
		log.Printf("[GetLog()] Error decoding pb.GetLogReply (key: %s) with gob: %v\n", key, err)
		return nil, status.Errorf(codes.Internal, "Error decoding pb.GetLogReply (key: %s)", key)
	}

	return logReply, nil
}

// Get all Logs by prefix
func (s *LogServer) GetLogs(ctx context.Context, req *pb.GetLogsRequest) (*pb.GetLogsReply, error) {
	prefix := []byte(req.GetPrefix())
	logs := []*pb.GetLogReply{}

	if err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(logsBucket)).Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			logReply, err := decodeLog(*bytes.NewBuffer(v))
			logReply.Key = string(k)
			if err != nil {
				log.Printf("[GetLogs()] Error decoding log (key: %s) : %v\n", k, err)
				return err
			}
			logs = append(logs, logReply)
		}
		return nil
	}); err != nil {
		log.Printf("[GetLogs()] Error reading from db: %v\n", err)
		return nil, status.Errorf(codes.Internal, "Error reading from db (prefix: %s)", prefix)
	}

	return &pb.GetLogsReply{
		Logs: logs,
	}, nil
}

// Stream Logs by prefix
func (s *LogServer) StreamLogs(req *pb.GetLogsRequest, stream pb.Logger_StreamLogsServer) error {
	c := s.getStreamChan(req.GetPrefix())

	for {
		select {
		case <-stream.Context().Done():
			return nil // return to not leak the goroutine
		case logReply := <-c:
			stream.Send(logReply)
		}
	}
	return nil
}

// List Log prefixes
func (s *LogServer) ListPrefixes(ctx context.Context, _ *empty.Empty) (*pb.ListPrefixesReply, error) {
	prefixes := []string{}

	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(prefixesBucket))
		b.ForEach(func(key, _ []byte) error {
			prefixes = append(prefixes, string(key))
			return nil
		})
		return nil
	}); err != nil {
		log.Printf("[ListPrefixes()] Error reading from db (bucket: %s): %v\n", prefixesBucket, err)
		return nil, status.Error(codes.Internal, "Error reading from db")
	}

	return &pb.ListPrefixesReply{
		Prefixes: prefixes,
	}, nil
}

// List Log keys by prefix
func (s *LogServer) ListLogs(ctx context.Context, req *pb.ListLogsRequest) (*pb.ListLogsReply, error) {
	prefix := []byte(req.GetPrefix())
	keys := []string{}

	if err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(logsBucket)).Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			keys = append(keys, string(k))
		}
		return nil
	}); err != nil {
		log.Printf("[ListLogs()] Error reading from db (prefix: %s): %v\n", prefix, err)
		return nil, status.Errorf(codes.Internal, "Error reading from db (prefix: %s)", prefix)
	}

	return &pb.ListLogsReply{
		Keys: keys,
	}, nil
}

func encodeLog(logReply *pb.GetLogReply) ([]byte, error) {
	var val bytes.Buffer
	enc := gob.NewEncoder(&val)
	if err := enc.Encode(logReply); err != nil {
		log.Printf("[encodeLog()] Error Encoding pb.GetLogReply with gob: %v\n", err)
		return nil, err
	}

	return val.Bytes(), nil
}

func decodeLog(buf bytes.Buffer) (*pb.GetLogReply, error) {
	logReply := &pb.GetLogReply{}
	dec := gob.NewDecoder(&buf)
	err := dec.Decode(logReply)
	if err != nil {
		log.Printf("[decodeLog()] Error Decoding pb.GetLogReply with gob: %v\n", err)
	}
	return logReply, err
}
