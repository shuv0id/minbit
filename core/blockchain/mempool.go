package blockchain

import (
	"sync"
)

type Mempool struct {
	transactions map[string]*Transaction
	mu           sync.Mutex
}

func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]*Transaction),
	}
}

func (m *Mempool) AddTx(tx *Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.transactions[tx.TxID]; exists {
		log.Info("Cannot add new transaction. Already exists in mempool", tx.TxID)
	}

	m.transactions[tx.TxID] = tx
	log.Info("Transaction added to the mempool", tx.TxID)
}

func (m *Mempool) RemoveTx(txID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.transactions[txID]; !exists {
		log.Info("Transaction not found in mempool", txID)
	} else {
		delete(m.transactions, txID)
	}
}
