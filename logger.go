package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"io"
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
	streams map[string](chan *pb.LogDetail)
}

func NewLogServer(db *bolt.DB) *LogServer {
	return &LogServer{
		db:      db,
		streams: make(map[string](chan *pb.LogDetail)),
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

func (s *LogServer) broadcastToStreams(prefix string, logDetail *pb.LogDetail) {
	if stream, ok := s.streams[prefix]; ok {
		stream <- logDetail
	}
}

func (s *LogServer) getStreamChan(prefix string) chan *pb.LogDetail {
	if stream, ok := s.streams[prefix]; ok {
		return stream
	}
	stream := make(chan *pb.LogDetail)
	s.streams[prefix] = stream
	return stream
}

// Write to Log
func (s *LogServer) PutLog(ctx context.Context, req *pb.PutLogRequest) (*pb.KeyMessage, error) {

	logDetail := &pb.LogDetail{
		Data:      req.GetData(),
		Timestamp: timestamp.TimestampNow(),
	}
	logDetail.Key = req.GetPrefix() + "|" + timestamp.TimestampString(logDetail.GetTimestamp())

	logBytes, encodeLogErr := encodeLog(logDetail)

	if encodeLogErr != nil {
		log.Printf("[Log()] Error encoding LogDetail with gob: %+v\n", encodeLogErr)
		return nil, status.Error(codes.Internal, "Error encoding LogDetail")
	}

	if dbErr := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(logsBucket))
		key := []byte(logDetail.GetKey())
		err := b.Put(key, logBytes)
		return err
	}); dbErr != nil {
		log.Printf("[Log()] Error saving to db: %+v\n", dbErr)
		return nil, status.Errorf(codes.Internal, "Error saving to db (key: %s)", logDetail.GetKey())
	}

	go s.broadcastToStreams(req.GetPrefix(), logDetail)

	go s.storePrefix(req.GetPrefix())

	return &pb.KeyMessage{
		Key: logDetail.GetKey(),
	}, nil
}

// Write to Log as stream
func (s *LogServer) PutLogStream(stream pb.Logger_PutLogStreamServer) error {

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err() // return to not leak the goroutine
		default:
		}

		req, recvErr := stream.Recv()
		if recvErr == io.EOF {
			return nil
		}
		if recvErr != nil {
			log.Printf("[PutLogStream()] Error in stream.Recv(): %+v\n", recvErr)
			return status.Errorf(codes.Internal, "Error receiving message from client.")
		}
		km, err := s.PutLog(stream.Context(), req)
		if err != nil {
			return err
		}
		if err := stream.Send(km); err != nil {
			log.Printf("[PutLogStream()] Error in stream.Send(%v): %+v\n", km, err)
			return status.Errorf(codes.Internal, "Error sending message to client.")
		}
	}

	return nil
}

// Get Log by key
func (s *LogServer) GetLog(ctx context.Context, req *pb.KeyMessage) (*pb.LogDetail, error) {
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

	logDetail, err := decodeLog(*bytes.NewBuffer(logReplyBytes))
	if err != nil {
		log.Printf("[GetLog()] Error decoding pb.LogDetail (key: %s) with gob: %v\n", key, err)
		return nil, status.Errorf(codes.Internal, "Error decoding pb.LogDetail (key: %s)", key)
	}

	return logDetail, nil
}

// Get all Logs by prefix
func (s *LogServer) GetLogs(ctx context.Context, req *pb.PrefixRequest) (*pb.LogDetailList, error) {
	prefix := []byte(req.GetPrefix())
	logs := []*pb.LogDetail{}

	if err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(logsBucket)).Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			logDetail, err := decodeLog(*bytes.NewBuffer(v))
			logDetail.Key = string(k)
			if err != nil {
				log.Printf("[GetLogs()] Error decoding log (key: %s) : %v\n", k, err)
				return err
			}
			logs = append(logs, logDetail)
		}
		return nil
	}); err != nil {
		log.Printf("[GetLogs()] Error reading from db: %v\n", err)
		return nil, status.Errorf(codes.Internal, "Error reading from db (prefix: %s)", prefix)
	}

	return &pb.LogDetailList{
		Logs: logs,
	}, nil
}

// Get all Logs as stream by prefix
func (s *LogServer) GetLogsStream(req *pb.PrefixRequest, stream pb.Logger_GetLogsStreamServer) error {
	prefix := []byte(req.GetPrefix())

	if err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(logsBucket)).Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			logDetail, err := decodeLog(*bytes.NewBuffer(v))
			logDetail.Key = string(k)
			if err != nil {
				log.Printf("[GetLogStream()] Error decoding log (key: %s) : %v\n", k, err)
				return err
			}

			stream.Send(logDetail)
		}
		return nil
	}); err != nil {
		log.Printf("[GetLogStream()] Error reading from db: %v\n", err)
		return status.Errorf(codes.Internal, "Error reading from db (prefix: %s)", prefix)
	}

	return nil
}

// Tail Logs as stream by prefix
func (s *LogServer) TailLogStream(req *pb.PrefixRequest, stream pb.Logger_TailLogStreamServer) error {
	c := s.getStreamChan(req.GetPrefix())

	for {
		select {
		case <-stream.Context().Done():
			return nil // return to not leak the goroutine
		case logDetail := <-c:
			stream.Send(logDetail)
		}
	}
	return nil
}

// List Log prefixes
func (s *LogServer) ListPrefixes(ctx context.Context, _ *empty.Empty) (*pb.PrefixesList, error) {
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

	return &pb.PrefixesList{
		Prefixes: prefixes,
	}, nil
}

// List Log keys by prefix
func (s *LogServer) ListKeys(ctx context.Context, req *pb.PrefixRequest) (*pb.KeysList, error) {
	prefix := []byte(req.GetPrefix())
	keys := []string{}

	if err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(logsBucket)).Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			keys = append(keys, string(k))
		}
		return nil
	}); err != nil {
		log.Printf("[ListKeys()] Error reading from db (prefix: %s): %v\n", prefix, err)
		return nil, status.Errorf(codes.Internal, "Error reading from db (prefix: %s)", prefix)
	}

	return &pb.KeysList{
		Keys: keys,
	}, nil
}

func encodeLog(logDetail *pb.LogDetail) ([]byte, error) {
	var val bytes.Buffer
	enc := gob.NewEncoder(&val)
	if err := enc.Encode(logDetail); err != nil {
		log.Printf("[encodeLog()] Error Encoding pb.LogDetail with gob: %v\n", err)
		return nil, err
	}

	return val.Bytes(), nil
}

func decodeLog(buf bytes.Buffer) (*pb.LogDetail, error) {
	logDetail := &pb.LogDetail{}
	dec := gob.NewDecoder(&buf)
	err := dec.Decode(logDetail)
	if err != nil {
		log.Printf("[decodeLog()] Error Decoding pb.LogDetail with gob: %v\n", err)
	}
	return logDetail, err
}
