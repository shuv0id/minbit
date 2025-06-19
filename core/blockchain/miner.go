package blockchain

import (
	"encoding/hex"
	"strings"
	"time"
)

type Miner struct {
	wallet      *Wallet
	blkRecEvent <-chan BlockRecEvent
}

func NewMiner(wallet *Wallet, blockRecEvent <-chan BlockRecEvent) (*Miner, error) {
	return &Miner{
		wallet:      wallet,
		blkRecEvent: blockRecEvent,
	}, nil
}

// CollectTransactions returns transaction(s) up to maxTxs pending from mempool
func (m *Miner) CollectTransactions(mem *Mempool, maxTxs int) []Transaction {
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
func (m *Miner) MineBlock(block *Block, difficulty int) *Block {
	prefix := strings.Repeat("0", difficulty)

	for i := 0; ; i++ {
		select {
		case incomingBlock := <-m.blkRecEvent:
			if incomingBlock.BlkHeight == block.Height {
				return nil
			}
		default:
			block.Nonce = i
			hash := block.calculateHash()
			if strings.HasPrefix(hash, prefix) {
				block.Hash = hash
				return block
			}
		}

		time.Sleep(200 * time.Millisecond) // putting a sleep for mocking compute-intensive mining process
	}
}

func (m *Miner) GenerateCoinbaseTx() Transaction {
	coinbaseReward := 6 // hardcoded block reward amount value; block reward halving not implemented
	var outputs []Output

	WalletPubKeyHash, err := AddressToPubKeyHash(m.wallet.Address)
	if err != nil {
		log.Error("Invalid Wallet address")
		return Transaction{}
	}

	rewardOutput := Output{
		Value:        coinbaseReward,
		ScriptPubKey: hex.EncodeToString(WalletPubKeyHash),
	}

	outputs = append(outputs, rewardOutput)
	coinbaseTx := Transaction{
		Amount:     coinbaseReward,
		Recipent:   m.wallet.Address,
		IsCoinbase: true,
		Outputs:    outputs,
		Timestamps: time.Now().String(),
	}
	coinbaseTx.TxID = hex.EncodeToString(coinbaseTx.Hash())
	return coinbaseTx
}
