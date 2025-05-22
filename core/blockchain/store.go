package blockchain

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

type bucketName string

const (
	blockBucket bucketName = "blocks"
	utxoBucket  bucketName = "utxos"
)

type Store struct {
	db          *bolt.DB
	blockBucket bucketName
	utxoBucket  bucketName
}

// NewDb opens a BoltDB instance at the specified path, returns error if operation fails
func NewDb(dbDir string) (*Store, error) {
	err := os.MkdirAll(dbDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "state.db")
	db, err := bolt.Open(dbPath, 0600, nil)

	if err != nil {
		return nil, fmt.Errorf("Error opening db: %v\n", err)
	}

	store := &Store{
		db:          db,
		blockBucket: blockBucket,
		utxoBucket:  utxoBucket,
	}

	return store, nil
}

// Creates a bucket for blocks if not already exists with name "Blocks"
func (store *Store) CreateBlocksBucket() error {
	err := store.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(blockBucket))
		if err != nil {
			return fmt.Errorf("Error creating bucket for blocks %v", err)
		}
		return nil
	})

	return err
}

// Creates a bucket for blocks if not already exists with name "UtxoSet"
func (store *Store) CreateUTXOSetBucket() error {
	err := store.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(utxoBucket))
		if err != nil {
			return fmt.Errorf("Error creating bucket for UtxoSet %v", err)
		}
		return nil
	})

	return err
}

func (store *Store) Db() *bolt.DB {
	return store.db
}

func (store *Store) BlocksBucket() bucketName {
	return store.blockBucket
}

func (store *Store) UTXOSetBucket() bucketName {
	return store.utxoBucket
}

func (store *Store) LoadBlocksFromTip() ([]*Block, error) {
	var blocks []*Block

	err := store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(store.blockBucket))
		if bucket == nil {
			return errors.New("bucket not found")
		}

		tip := bucket.Get([]byte("tip"))
		if tip == nil {
			return nil
		}

		hash := tip
		for hash != nil && len(hash) > 0 {
			blockBytes := bucket.Get(hash)
			block, err := deserializeBlock(blockBytes)
			if err != nil {
				return err
			}
			blocks = append(blocks, &block)
			if block.PrevHash == "" {
				break
			}
			hash = []byte(block.PrevHash)
		}
		return nil
	})

	// Reverse to get correct order
	for i, j := 0, len(blocks)-1; i < j; i, j = i+1, j-1 {
		blocks[i], blocks[j] = blocks[j], blocks[i]
	}

	return blocks, err
}

func (store *Store) WriteBlock(block *Block) error {
	return store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(store.blockBucket))
		if bucket == nil {
			return errors.New("block bucket not found")
		}

		data, err := serializeBlock(*block)
		if err != nil {
			return err
		}

		if err := bucket.Put([]byte(block.Hash), data); err != nil {
			return err
		}
		if err := bucket.Put([]byte("tip"), []byte(block.Hash)); err != nil {
			return err
		}
		return nil
	})
}

func (store *Store) Close() error {
	if err := store.db.Close(); err != nil {
		return err
	}
	return nil
}

func serializeBlock(b Block) ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(b)
	return buf.Bytes(), err
}

func deserializeBlock(data []byte) (Block, error) {
	var b Block
	err := gob.NewDecoder(bytes.NewReader(data)).Decode(&b)
	return b, err
}
