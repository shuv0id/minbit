package blockchain

type Input struct {
	PrevTxID    string
	OutputIndex int
	Value       int
}

type Output struct {
	OutputIndex int
	Value       int
	Address     string
}

type UTXOSet struct {
	UTXOs map[string]map[int]Output // map of transaction id mapped to output indexes mapped to Output
}

var us = &UTXOSet{
	UTXOs: make(map[string]map[int]Output),
}

func (us *UTXOSet) AddUTXO(txID string, outputIndex int, amount int, address string) {
	if _, exists := us.UTXOs[txID]; !exists {
		us.UTXOs[txID] = make(map[int]Output)
	}
	us.UTXOs[txID][outputIndex] = Output{OutputIndex: outputIndex, Value: amount, Address: address}
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
	for _, transactions := range us.UTXOs {
		for _, utxo := range transactions {
			if utxo.Address == address {
				totalUTXOs += utxo.Value
			}
		}
	}
	return totalUTXOs
}

func GetUTXOsByAddress(address string) []Output {
	var utxos []Output
	for _, transactions := range us.UTXOs {
		for _, utxo := range transactions {
			if utxo.Address == address {
				utxos = append(utxos, utxo)
			}
		}
	}
	return utxos
}

func GetInputsForTxByAddress(address string) []Input {
	var inputs []Input
	for txID, transactions := range us.UTXOs {
		for outputIndex, utxo := range transactions {
			if utxo.Address == address {
				input := Input{
					PrevTxID:    txID,
					OutputIndex: outputIndex,
					Value:       utxo.Value,
				}
				inputs = append(inputs, input)
			}
		}
	}
	return inputs

}
