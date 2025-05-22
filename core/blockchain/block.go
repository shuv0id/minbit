package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/shu8h0-null/minbit/core/logger"
)

var log = logger.NewLogger()

type Block struct {
	Height     int           `json:"height"`
	TxData     []Transaction `json:"transaction_data"`
	Timestamps string        `json:"timestamps"`
	Nonce      int           `json:"nonce"`
	Hash       string        `json:"hash"`
	PrevHash   string        `json:"prev_hash"`
}

type Blockchain struct {
	chain      []*Block
	difficulty int // Mining Difficulty. In real blockchains, this is adjusted dynamically over time.
	store      *Store
	mu         sync.Mutex
}

// NewBlockchain initializes a Blockchain with the given BoltDB instance.
// It loads existing blocks from the database and returns the Blockchain.
func NewBlockchain(store *Store, blockBucket string) (*Blockchain, error) {
	bc := &Blockchain{
		difficulty: 2, // hard-coded difficulty; actual blockchain adjust difficulty dynamically over-time
		store:      store,
	}

	err := bc.Load()

	return bc, err
}

func NewBlock() *Block {
	return &Block{}
}

// Load loads all blocks from the db
// Returns error if Db for blockchain is not initialised or if the transactions fails
func (bc *Blockchain) Load() error {
	if bc.store == nil {
		return errors.New("Cannot load blockchain. Db is not initialised with a blockchain")
	}

	store := bc.Store()
	blocks, err := store.LoadBlocksFromTip()
	if err != nil {
		return fmt.Errorf("Error loading blocks from db: %v\n", err)
	}

	// Reverse the obtained block slice to get the correct order
	for i, j := 0, len(blocks)-1; i < j; i, j = i+1, j-1 {
		blocks[i], blocks[j] = blocks[j], blocks[i]
	}

	bc.chain = append(bc.chain, blocks...)

	return err
}

// AddBlock add the new block to the blockchain
func (bc *Blockchain) AddBlock(block *Block) error {

	err := RetryN(func() error {
		err := bc.Store().WriteBlock(block)
		return err
	}, 3, fmt.Sprintf("Error writing block:[%s] to db", block.Hash))

	// Add block to in-memory chain after successful write of the block to the db
	if err == nil {
		bc.mu.Lock()
		defer bc.mu.Unlock()
		bc.chain = append(bc.chain, block)
	}

	return err
}

func (bc *Blockchain) NewBlock(txs []Transaction) *Block {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	var blockHeight int
	var pvHash string

	if len(bc.chain) == 0 {
		blockHeight = 0
		pvHash = ""
	} else {
		blockHeight = bc.chain[len(bc.chain)-1].Height + 1
		pvHash = bc.chain[len(bc.chain)-1].Hash
	}

	b := Block{
		Height:     blockHeight,
		TxData:     txs,
		Timestamps: time.Now().String(),
		PrevHash:   pvHash,
	}
	return &b
}

func (bc *Blockchain) Store() *Store {
	return bc.store
}

func (bc *Blockchain) Chain() []*Block {
	return bc.chain
}

func (bc *Blockchain) Difficulty() int {
	return bc.difficulty
}

func (bc *Blockchain) GetBlockchainHeight() int {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	chainLen := len(bc.chain)
	if chainLen == 0 {
		return -1
	}
	return bc.chain[chainLen-1].Height
}
func (bc *Blockchain) IsValid(b *Block) bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if len(bc.chain) > 0 {
		latestBlock := bc.chain[len(bc.chain)-1]
		if latestBlock.Height+1 != b.Height {
			log.Error("Block validation failed: invalid block height")
			return false
		}
		if latestBlock.Hash != b.PrevHash {
			log.Error("Block validation failed: previous block hash not matched")
			return false
		}
	}
	if !b.validateHash() {
		log.Error("Block validation failed: invalid hash")
		return false
	}

	return true
}

func (b *Block) validateHash() bool {
	h := b.calculateHash()
	if h != b.Hash {
		return false
	}
	return true
}

func (b *Block) calculateHash() string {
	data := strconv.Itoa(b.Height) + TransactionsToString(b.TxData) + b.Timestamps + strconv.Itoa(b.Nonce) + b.PrevHash

	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:])
}
