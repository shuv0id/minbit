package blockchain

import (
	"strings"
	"time"
)

type Miner struct {
	wallet  *Wallet
	blockCh <-chan *Block
}

func NewMiner(wallet *Wallet, blockCh <-chan *Block) (*Miner, error) {
	return &Miner{
		wallet:  wallet,
		blockCh: blockCh,
	}, nil
}

// CollectTransactions returns transaction(s) up to maxTxs pending from mempool
func (miner *Miner) CollectTransactions(mem *Mempool, maxTxs int) []Transaction {
	txCount := 0
	var txs []Transaction
	for _, tx := range mem.transactions {
		if txCount >= maxTxs {
			break
		}
		txs = append(txs, *tx)
		txCount++
	}
	return txs
}

// MineBlock performs proof-of-work for a block, returns true if block is mined, false if
func (miner *Miner) MineBlock(block *Block, difficulty int) (*Block, bool) {
	prefix := strings.Repeat("0", difficulty)

	for i := 0; ; i++ {
		block.Nonce = i
		hash := block.calculateHash()
		if strings.HasPrefix(hash, prefix) {
			block.Hash = hash
			log.Infof("Block:[%d]:[%s] mined", block.Height, hash)
			return block, true
		}

		select {
		case incomingBlock := <-miner.blockCh:
			if incomingBlock.Height == block.Height {
				return incomingBlock, false
			}
		default:
		}

		time.Sleep(200 * time.Millisecond) // putting a sleep for mocking compute-intensive mining process
	}
}
