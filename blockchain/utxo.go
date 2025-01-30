package blockchain

import (
	"encoding/hex"
	"errors"
)

// Input represents input in a transaction
type Input struct {
	PrevTxID    string `json:"prev_tx_id"`
	OutputIndex int    `json:"output_index"`
	ScriptSig   string `json:"script_sig"`
}

// Output represents output in a transaction
type Output struct {
	Value        int
	ScriptPubKey string
}

// UTXO is used to track unspent outputs
type UTXO struct {
	TxID         string
	OutputIndex  int
	Value        int
	ScriptPubKey string
}

type UTXOSet struct {
	UTXOs map[string]map[int]UTXO // map of transaction id mapped to output indexes mapped to UTXO
}

var utxoSet = &UTXOSet{
	UTXOs: make(map[string]map[int]UTXO),
}

func (us *UTXOSet) addUTXO(txID string, outputIndex int, amount int, scriptPubKey string) {
	if _, exists := us.UTXOs[txID]; !exists {
		us.UTXOs[txID] = make(map[int]UTXO)
	}
	us.UTXOs[txID][outputIndex] = UTXO{
		TxID:         txID,
		OutputIndex:  outputIndex,
		Value:        amount,
		ScriptPubKey: scriptPubKey,
	}
}

func (us *UTXOSet) removeUTXO(txID string, outputIndex int) {
	if utxos, exists := us.UTXOs[txID]; exists {
		delete(utxos, outputIndex)
		if len(utxos) == 0 {
			delete(us.UTXOs, txID)
		}
	}
}

func (us *UTXOSet) update(txs []Transaction) {
	for _, tx := range txs {
		for _, input := range tx.Inputs {
			us.removeUTXO(tx.TxID, input.OutputIndex)
		}
		for index, output := range tx.Outputs {
			us.addUTXO(tx.TxID, index, output.Value, output.ScriptPubKey)
		}
	}
}

func (us *UTXOSet) GetUTXO(txID string, outIndex int) (UTXO, error) {
	var utxo UTXO
	for transactionID, transactions := range us.UTXOs {
		if transactionID == txID {
			for outputIndex, utxo := range transactions {
				if outputIndex == outIndex {
					return utxo, nil
				}
			}
		}
	}
	return utxo, errors.New("UTXO not found with specified transaction id and output index.")
}

func (us *UTXOSet) GetTotalBalByAddress(address string) int {
	var totalBal int
	for _, transactions := range us.UTXOs {
		for _, utxo := range transactions {
			pubKeyHash, err := hex.DecodeString(utxo.ScriptPubKey)
			if err != nil {
				return 0
			}

			utxoAddr := PubKeyHashToAddress(pubKeyHash)
			if utxoAddr == address {
				totalBal += utxo.Value
			}
		}
	}
	return totalBal
}

// GetAvailableIns returns slice of Input available to a address
func (us *UTXOSet) GetAvailableUTXOS(address string) []UTXO {
	var utxos []UTXO
	for _, transactions := range us.UTXOs {
		for _, output := range transactions {
			pubKeyHash, err := hex.DecodeString(output.ScriptPubKey)
			if err != nil {
				logger.Error("Error decoding scriptPubKey", err)
				return utxos
			}

			utxoAddr := PubKeyHashToAddress(pubKeyHash)
			if utxoAddr == address {
				utxos = append(utxos, output)
			}
		}
	}
	return utxos
}

// ResolveInputs selects enough inputs to meet the specified amount to be sent.
// Returns an error if the sum of inputs is insufficient.
func ResolveInputs(totalUTXOs []UTXO, amtToBeSent int) ([]Input, error) {
	var inputs []Input
	inputAmtSum := 0

	if len(totalUTXOs) == 0 {
		return nil, errors.New("no UTXOs available to spend")
	}

	for _, utxo := range totalUTXOs {
		if inputAmtSum <= amtToBeSent {
			input := Input{
				PrevTxID:    utxo.TxID,
				OutputIndex: utxo.OutputIndex,
			}
			inputs = append(inputs, input)
			inputAmtSum += utxo.Value
		} else {
			break
		}
	}

	if inputAmtSum < amtToBeSent {
		return nil, errors.New("insufficent total UTXOs to cover the amount to be sent")
	}
	return inputs, nil
}

// DeriveOutputs generates transaction outputs based on inputs, amount, recipient, and sender addresses.
// It creates up to two outputs: one for the recipient and, if needed, another for the sender's remaining balance.
func DeriveOutputs(us UTXOSet, inputs []Input, amount int, recipAddr, senderAddr string) []Output {
	totalInputAmount := 0
	var outputs []Output
	for _, inp := range inputs {
		utxo, err := us.GetUTXO(inp.PrevTxID, inp.OutputIndex)

		if err != nil {
			logger.Error("Invalid input, found no corressponding output", err)
			return outputs
		}

		totalInputAmount += utxo.Value
	}

	senderPubKeyHash, err := AddressToPubKeyHash(senderAddr)
	if err != nil {
		logger.Error("Invalid sender address")
		return outputs
	}
	recipAddrPubKeyHash, err := AddressToPubKeyHash(senderAddr)
	if err != nil {
		logger.Error("Invalid recipent address")
		return outputs
	}

	if totalInputAmount > amount {
		// we can have two outputs at max in our implementation since in a single transaction one address can send to only one address
		// thus resulting two outputs at max -> one for the receipent and the other output(if applicable) for the remaining amount for the sender itself
		output1 := Output{
			Value:        totalInputAmount - amount,
			ScriptPubKey: hex.EncodeToString(senderPubKeyHash),
		}
		output2 := Output{
			Value:        amount,
			ScriptPubKey: hex.EncodeToString(recipAddrPubKeyHash),
		}
		outputs = append(outputs, output1, output2)
	} else if totalInputAmount == amount {
		output := Output{
			Value:        amount,
			ScriptPubKey: recipAddr,
		}
		outputs = append(outputs, output)
	}
	return outputs
}
