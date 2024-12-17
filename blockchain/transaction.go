package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strconv"

	"github.com/mr-tron/base58/base58"
)

type Transaction struct {
	TxID      string `json:"trasaction_id"`
	Sender    string `json:"sender"`
	Recipent  string `json:"recipent"`
	Amount    int    `json:"amount"`
	Signature string `json:"signature"`
	Inputs    []UTXO `json:"inputs"`
	Outputs   []UTXO `json:"outputs"`
}

func (tx *Transaction) isValid() bool {
	pbKeyAddr, _ := PublickKeyToAddress(tx.Sender)
	if len(tx.Inputs) == 0 || len(tx.Outputs) == 0 || tx.Amount > us.GetTotalUTXOsByAddress(pbKeyAddr) {
		return false
	}

	pubKeyBytes, err := base58.Decode(tx.Sender)
	if err != nil {
		logger.Error("Invalid public key")
		return false
	}
	sigBytes, err := base58.Decode(tx.Signature)
	if err != nil {
		logger.Error("Invalid signature")
	}

	publicKey := ecdsa.PublicKey{
		Curve: elliptic.P224(),
		X:     new(big.Int).SetBytes(pubKeyBytes[:32]),
		Y:     new(big.Int).SetBytes(pubKeyBytes[32:]),
	}

	hash := tx.Hash()

	if !ecdsa.VerifyASN1(&publicKey, hash, sigBytes[:]) {
		return false
	}

	return true
}

func (tx *Transaction) Hash() []byte {
	txData := tx.TxID + tx.Sender + tx.Recipent + strconv.Itoa(tx.Amount)

	for _, input := range tx.Inputs {
		txData += input.TxID + strconv.Itoa(input.Amount) + strconv.Itoa(input.OutputIndex) + input.Address
	}

	for _, output := range tx.Outputs {
		txData += output.TxID + strconv.Itoa(output.Amount) + strconv.Itoa(output.OutputIndex) + output.Address
	}

	hash := sha256.Sum256([]byte(txData))

	return hash[:]
}

func (tx Transaction) String() string {
	return fmt.Sprintf("%s %s %d %s", tx.Sender, tx.Recipent, tx.Amount, tx.Signature)
}

func GetUserTxHistory(walletAddress string) []Transaction {
	var txHistory []Transaction
	for _, b := range bc.Chain {
		if b.TxData.Sender == walletAddress || b.TxData.Recipent == walletAddress {
			txHistory = append(txHistory, b.TxData)
		}
	}
	return txHistory
}
