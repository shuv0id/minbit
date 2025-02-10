package blockchain

import (
	"fmt"
	"os"

	bolt "go.etcd.io/bbolt"
)

type bucketName string

const (
	blockBucket bucketName = "Blocks"
	utxoBucket  bucketName = "UtxoSet"
)

// InitDB opens a BoltDB instance at the specified path and creates the required
// buckets for the blockchain and UTXO set. It returns the opened database handle,
// or an error if any step fails.
func InitDB() (*bolt.DB, error) {
	dbDir := fmt.Sprintf("./store/node-%s", node.ID().String())

	err := os.MkdirAll(dbDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := fmt.Sprintf("%s/state.db", dbDir)
	db, err := bolt.Open(dbPath, 0600, nil)

	if err != nil {
		return nil, fmt.Errorf("Error opening db: %v\n", err)
	}

	err = createBlocksBucket(db)
	if err != nil {
		return nil, fmt.Errorf("Error creating bucket for blocks: %v\n", err)
	}

	err = createUTXOSetBucket(db)
	if err != nil {
		return nil, fmt.Errorf("Error creating bucket for UtxoSet: %v\n", err)
	}

	return db, nil
}

// Creates a bucket for blocks if not already exists with name "Blocks"
func createBlocksBucket(db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(blockBucket))
		tx.Bucket([]byte("Blocks")).Cursor()
		if err != nil {
			return fmt.Errorf("Error creating bucket for blocks %v", err)
		}
		return nil
	})

	return err
}

// Creates a bucket for blocks if not already exists with name "UtxoSet"
func createUTXOSetBucket(db *bolt.DB) error {
	err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(utxoBucket))
		if err != nil {
			return fmt.Errorf("Error creating bucket for UtxoSet %v", err)
		}
		return nil
	})

	return err
}
