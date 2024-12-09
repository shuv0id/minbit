package blockchain

import (
	"sync"
)

type Mempool struct {
	transactions map[string]*Transaction
	mu           sync.Mutex
}

var mempool = &Mempool{transactions: make(map[string]*Transaction)}

func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]*Transaction),
	}
}

func (m *Mempool) AddTransaction(tx *Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.transactions[tx.TxID]; exists {
		logger.Info("Cannot add new transaction. Already exists in mempool", tx.TxID)
	}

	m.transactions[tx.TxID] = tx
	logger.Info("Transaction added to the mempool", tx.TxID)
}

func (m *Mempool) RemoveTransaction(txID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.transactions[txID]; !exists {
		logger.Info("Transaction not found in mempool", txID)
	} else {
		delete(m.transactions, txID)
	}
}
