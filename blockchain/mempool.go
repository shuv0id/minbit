package blockchain

import (
	"log"
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
		log.Println("Transaction already exists in mempool", tx.TxID)
	}

	m.transactions[tx.TxID] = tx
	log.Println("Transaction added to the mempool", tx.TxID)
}

func (m *Mempool) RemoveTransaction(txID string) {
	m.mu.Lock()
	defer m.mu.Lock()

	if _, exists := m.transactions[txID]; !exists {
		log.Println("Transaction not found in mempool", txID)
	} else {
		delete(m.transactions, txID)
	}
}
