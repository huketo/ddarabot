package store

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketProcessed = []byte("processed_posts")
	bucketCursor    = []byte("cursor")
	keyCursor       = []byte("jetstream_cursor")
)

type processedRecord struct {
	Timestamp int64    `json:"timestamp"`
	Languages []string `json:"languages"`
}

type Store struct {
	db *bolt.DB
}

func New(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketProcessed); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(bucketCursor); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create buckets: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) IsProcessed(uri string) bool {
	var found bool
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketProcessed)
		found = b.Get([]byte(uri)) != nil
		return nil
	})
	return found
}

func (s *Store) MarkProcessed(uri string, languages []string) error {
	rec := processedRecord{
		Timestamp: time.Now().Unix(),
		Languages: languages,
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketProcessed).Put([]byte(uri), val)
	})
}

func (s *Store) SaveCursor(cursor int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketCursor).Put(keyCursor, []byte(strconv.FormatInt(cursor, 10)))
	})
}

func (s *Store) GetCursor() (int64, bool) {
	var cursor int64
	var found bool
	s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketCursor)
		v := b.Get(keyCursor)
		if v != nil {
			c, err := strconv.ParseInt(string(v), 10, 64)
			if err == nil {
				cursor = c
				found = true
			}
		}
		return nil
	})
	return cursor, found
}
