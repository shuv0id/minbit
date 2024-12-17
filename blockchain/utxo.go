package blockchain

type UTXO struct {
	TxID        string
	OutputIndex int
	Amount      int
	Address     string
}

type UTXOSet struct {
	UTXOs map[string]map[int]UTXO
}

var us = &UTXOSet{
	UTXOs: make(map[string]map[int]UTXO),
}

func (us *UTXOSet) AddUTXO(txID string, outputIndex int, amount int, address string) {
	if _, exists := us.UTXOs[txID]; !exists {
		us.UTXOs[txID] = make(map[int]UTXO)
	}
	us.UTXOs[txID][outputIndex] = UTXO{TxID: txID, OutputIndex: outputIndex, Amount: amount, Address: address}
}

func (us *UTXOSet) Remove(txID string, outputIndex int, address string) {
	if outputs, exists := us.UTXOs[txID]; exists {
		delete(outputs, outputIndex)
		if len(outputs) == 0 {
			delete(us.UTXOs, txID)
		}
	}
}

func (us *UTXOSet) GetTotalUTXOsByAddress(address string) int {
	var totalUTXOs int
	for _, outputs := range us.UTXOs {
		for _, utxo := range outputs {
			if utxo.Address == address {
				totalUTXOs += utxo.Amount
			}
		}
	}
	return totalUTXOs
}
