package storage

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/boltdb/bolt"
)

type Store struct {
	Key     string
	DB      *bolt.DB
	Buckets map[string]interface{}
}

var store = make(map[string]*Store)

//Loads a BoldBD Store given key string
//If DB doesn't exist a new one is created.
//You can load it many times throughout your project
//it will look in memory first before loading from a file
func Load(key string) (*Store, error) {
	if strings.ContainsAny(key, " ") {
		return nil, errors.New("storage key cannot contain spaces")
	}
	log.Printf("Loading %v storage", key)
	if store[key] == nil {
		s := Store{
			Key: key,
		}
		storagefile := fmt.Sprintf("./storage/%v.db", key)
		db, err := bolt.Open(storagefile, 0600, nil)
		if err != nil {
			return &s, err
		}
		s.DB = db
		s.Buckets = make(map[string]interface{})
		store[key] = &s
		return &s, nil
	} else {
		return store[key], nil
	}
}

//Loads Bucket by name, Adds bucket if it doesn't exist in the store
func (s *Store) LoadBucket(name string) error {
	name = strings.ToLower(name)
	if strings.ContainsAny(name, " ") {
		return errors.New("storage bucket cannot contain spaces")
	}

	if s.Buckets[name] != nil {
		return errors.New("bucket already exists")
	}
	if err := s.DB.Batch(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			return err
		}
		s.Buckets[name] = true
		return nil
	}); err != nil {
		return err
	}
	return nil
}

//Get value from specified bucket and key
func (s *Store) GetVal(bucket string, key string) (string, error) {
	var val string
	key = strings.ToLower(key)
	bucket = strings.ToLower(bucket)
	if strings.ContainsAny(bucket, " ") {
		return val, errors.New("storage bucket cannot contain spaces")
	}
	if s.Buckets[bucket] == nil {
		return val, errors.New("bucket doesn't exist")
	}
	if strings.ContainsAny(key, " ") {
		return val, errors.New("storage key cannot contain spaces")
	}

	err := s.DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket)).Get([]byte(key))
		if len(b) > 0 {
			val = string(b)
			return nil
		}
		return errors.New("no value")
	})
	return val, err
}

//Set value to specified bucket and key
func (s *Store) SetVal(bucket string, key string, value string) error {
	bucket = strings.ToLower(bucket)
	key = strings.ToLower(key)
	if strings.ContainsAny(bucket, " ") {
		return errors.New("storage bucket cannot contain spaces")
	}
	if s.Buckets[bucket] == nil {
		return errors.New("bucket doesn't exist")
	}
	if strings.ContainsAny(key, " ") {
		return errors.New("storage key cannot contain spaces")
	}

	return s.DB.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Put([]byte(key), []byte(value))
	})
}
