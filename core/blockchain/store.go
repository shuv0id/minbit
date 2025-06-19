package blockchain

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/shu8h0-null/minbit/core/config"
	bolt "go.etcd.io/bbolt"
)

type bucketName string

const (
	blockBucket   bucketName = "blocks"
	utxoBucket    bucketName = "utxos"
	txIndexBucket bucketName = "txIndex"
)

type Store struct {
	db            *bolt.DB
	blockBucket   bucketName
	utxoBucket    bucketName
	txIndexBucket bucketName
}

// NewDb opens a BoltDB instance, returns error if operation fails
func NewDb(id string) (*Store, error) {
	dbDir := filepath.Join(config.StoreDir(), id)
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

// Creates a bucket for blocks if not already exists with name "blocks"
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

// Creates a bucket for blocks if not already exists with name "utxoSet"
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

// Creates a bucket for blocks if not already exists with name "txIndex"
func (store *Store) CreateTxIndexBucket() error {
	err := store.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(txIndexBucket))
		if err != nil {
			return fmt.Errorf("Error creating bucket for TxIndex %v", err)
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

// WriteUpdate writes the changes to the db by deleting spent outputs and adding new ones with the provided transaction
// Returns an error if any database operation fails.
func (store *Store) WriteUTXOs(transaction Transaction) error {
	err := store.Db().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(store.UTXOSetBucket()))
		for _, input := range transaction.Inputs {
			k := input.PrevTxID + "_" + strconv.Itoa(input.OutputIndex)
			if err := b.Delete([]byte(k)); err != nil {
				return err
			}
		}

		for index, output := range transaction.Outputs {
			var v bytes.Buffer
			k := transaction.TxID + "_" + strconv.Itoa(index)
			gob.NewEncoder(&v).Encode(output)
			if err := b.Put([]byte(k), v.Bytes()); err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func (store *Store) LoadUTXOs() (UTXOMap, error) {
	var umap = make(UTXOMap)

	err := store.Db().View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(store.UTXOSetBucket()))
		if b == nil {
			return errors.New("utxo bucket not found")
		}

		err := b.ForEach(func(k, v []byte) error {
			u, err := deserializeUTXO(v)

			if _, exists := umap[u.TxID]; !exists {
				umap[u.TxID] = make(map[int]UTXO)
			}
			umap[u.TxID][u.OutputIndex] = UTXO{
				TxID:         u.TxID,
				OutputIndex:  u.OutputIndex,
				Value:        u.Value,
				ScriptPubKey: u.ScriptPubKey,
			}
			return err
		})

		return err
	})

	return umap, err
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

func serializeUTXO(u UTXO) ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(u)
	return buf.Bytes(), err
}

func deserializeUTXO(data []byte) (UTXO, error) {
	var u UTXO
	err := gob.NewDecoder(bytes.NewReader(data)).Decode(&u)
	return u, err
}

func serializeTx(tx Transaction) ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(tx)
	return buf.Bytes(), err
}

func deserializeTx(data []byte) (Transaction, error) {
	var tx Transaction
	err := gob.NewDecoder(bytes.NewReader(data)).Decode(&tx)
	return tx, err
}
