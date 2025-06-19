package blockchain

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/mr-tron/base58/base58"
)

type Transaction struct {
	TxID       string   `json:"transaction_id"`
	Sender     string   `json:"sender"`
	Recipent   string   `json:"recipent"`
	Amount     int      `json:"amount"`
	Inputs     []Input  `json:"inputs"`
	Outputs    []Output `json:"outputs"`
	Timestamps string   `json:"timestamps"`
	IsCoinbase bool     `json:"is_coinbase"`
}

func (tx *Transaction) Sign(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	hash := tx.Hash()
	tx.TxID = hex.EncodeToString(hash)

	sigBytes, err := ecdsa.SignASN1(rand.Reader, privateKey, hash)
	if err != nil {
		return nil, err
	}

	return sigBytes, nil
}

func (tx *Transaction) IsValid() bool {
	_, err := hex.DecodeString(tx.TxID)
	if err != nil {
		log.Error("Invalid transaction id", err)
		return false
	}

	_, err = base58.Decode(tx.Sender)
	if err != nil {
		log.Error("Invalid Sender Address", err)
		return false
	}
	_, err = base58.Decode(tx.Recipent)
	if err != nil {
		log.Error("Invalid Recipent Address", err)
		return false
	}

	if len(tx.Outputs) == 0 {
		log.Info("Invalid number of output: ", len(tx.Outputs))
		return false
	}

	// NOTE: thinking ....should i only validate transaction fields or also check them against utxoSet here?
	// if tx.Amount > utxoSet.GetTotalBalByAddress(tx.Sender) {
	// 	log.Info("Balance insufficient to process the transaction")
	// }

	// if !tx.IsCoinbase {
	// 	for _, input := range tx.Inputs {
	// 		utxo, err := utxoSet.GetUTXO(input.PrevTxID, input.OutputIndex)
	// 		if err != nil {
	// 			log.Error("Invalid input, found no corressponding output", err)
	// 		}
	//
	// 		if err := UnlockUTXO(input, utxo, txIDBytes); err != nil {
	// 			return false
	// 		}
	// 	}
	//
	// }

	return true
}

// Hash hashes the transaction data leaving the txID and scriptSig of each inputs
func (tx *Transaction) Hash() []byte {
	txData := tx.Sender + tx.Recipent + strconv.Itoa(tx.Amount) + tx.Timestamps + strconv.FormatBool(tx.IsCoinbase)

	for _, input := range tx.Inputs {
		txData += input.PrevTxID + strconv.Itoa(input.OutputIndex)
	}

	for _, output := range tx.Outputs {
		txData += strconv.Itoa(output.Value) + output.ScriptPubKey
	}

	hash := sha256.Sum256([]byte(txData))

	return hash[:]
}

func TransactionsToString(transactions []Transaction) string {
	txStr := []string{}
	for _, tx := range transactions {
		inputsStr := []string{}
		for _, input := range tx.Inputs {
			inputsStr = append(inputsStr, input.PrevTxID+", "+strconv.Itoa(input.OutputIndex)+", ")
		}

		outputsStr := []string{}
		for _, output := range tx.Outputs {
			outputsStr = append(outputsStr, strconv.Itoa(output.Value)+", "+output.ScriptPubKey)
		}

		txStr = append(txStr, strconv.FormatBool(tx.IsCoinbase)+", "+strings.Join(inputsStr, ", ")+
			", "+strings.Join(outputsStr, ", "))
	}

	return strings.Join(txStr, "\n")
}
