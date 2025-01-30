package blockchain

import (
	"encoding/hex"
	"errors"
)

type Input struct {
	PrevTxID    string `json:"prev_tx_id"`
	OutputIndex int    `json:"output_index"`
	ScriptSig   string `json:"script_sig"`
	Value       int    `json:"value"`
}

type Output struct {
	Value        int
	ScriptPubKey string
}

type UTXOSet struct {
	UTXOs map[string]map[int]Output // map of transaction id mapped to output indexes mapped to Output
}

var utxoSet = &UTXOSet{
	UTXOs: make(map[string]map[int]Output),
}

func (us *UTXOSet) addUTXO(txID string, outputIndex int, amount int, scriptPubKey string) {
	if _, exists := us.UTXOs[txID]; !exists {
		us.UTXOs[txID] = make(map[int]Output)
	}
	us.UTXOs[txID][outputIndex] = Output{Value: amount, ScriptPubKey: scriptPubKey}
}

func (us *UTXOSet) removeUTXO(txID string, outputIndex int) {
	if outputs, exists := us.UTXOs[txID]; exists {
		delete(outputs, outputIndex)
		if len(outputs) == 0 {
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

func (us *UTXOSet) getUTXO(txID string, outIndex int) (Output, error) {
	var output Output
	for transactionID, transactions := range us.UTXOs {
		if transactionID == txID {
			for outputIndex, output := range transactions {
				if outputIndex == outIndex {
					return output, nil
				}
			}
		}
	}
	return output, errors.New("No output found with specified transaction io and output index.")
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

func GetAvailableIns(address string) []Input {
	var inputs []Input
	for txID, transactions := range utxoSet.UTXOs {
		for outputIndex, utxo := range transactions {
			pubKeyHash, err := hex.DecodeString(utxo.ScriptPubKey)
			if err != nil {
				logger.Error("Error decoding scriptPubKey", err)
				return inputs
			}

			utxoAddr := PubKeyHashToAddress(pubKeyHash)
			if utxoAddr == address {
				input := Input{
					PrevTxID:    txID,
					OutputIndex: outputIndex,
				}
				inputs = append(inputs, input)
			}
		}
	}
	return inputs
}

// ResolveInputs selects enough inputs to meet the specified amount to be sent.
// Returns an error if the sum of inputs is insufficient.
func ResolveInputs(totalIns []Input, amtToBeSent int) ([]Input, error) {
	var inputs []Input
	inputAmtSum := 0

	if len(totalIns) == 0 {
		return nil, errors.New("no UTXOs available to spend")
	}

	for _, input := range totalIns {
		if inputAmtSum <= amtToBeSent {
			inputs = append(inputs, input)
			inputAmtSum += input.Value
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
func DeriveOutputs(inputs []Input, amount int, recipAddr, senderAddr string) []Output {
	totalInputAmount := 0
	var outputs []Output
	for _, inp := range inputs {
		totalInputAmount += inp.Value
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
