package storage

import (
	"errors"
	"fmt"
	"log"

	"github.com/boltdb/bolt"
)

type Store struct {
	Key     string
	DB      *bolt.DB
	Buckets map[string]*bolt.Bucket
}

//Loads a BoldBD Store given key string
//If DB doesn't exist a new one is created.
func Load(key string) (*Store, error) {
	log.Printf("Loading %v storage", key)
	s := Store{
		Key: key,
	}
	storagefile := fmt.Sprintf("./storage/%v.db", key)
	db, err := bolt.Open(storagefile, 0600, nil)
	if err != nil {
		return &s, err
	}
	s.DB = db
	s.DB.Batch(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, buck *bolt.Bucket) error {
			s.Buckets[string(name)] = buck
			return nil
		})
	})
	return &Store{}, nil
}

//Adds Bucket to the DB Store
func (s *Store) AddBucket(name string) error {
	if s.Buckets[name] != nil {
		return errors.New("bucket already exists")
	}
	if err := s.DB.Batch(func(tx *bolt.Tx) error {
		buck, err := tx.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			return err
		}
		s.Buckets[name] = buck
		return nil
	}); err != nil {
		return err
	}
	return nil
}
