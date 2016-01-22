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
	return &Store{}, nil
}

//Loads Bucket by name, Adds bucket if it doesn't exist in the store
func (s *Store) LoadBucket(name string) error {
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

//Get value from specified bucket and key
func (s *Store) GetVal(bucket string, key string) (string, error) {
	if r := s.Buckets[bucket].Get([]byte(key)); len(r) > 0 {
		return r, nil
	}
	return "", nil
}

//Set value to specified bucket and key
func (s *Store) SetVal(bucket string, key string, value string) error {
	return s.Buckets[bucket].Put(key, value)
}
