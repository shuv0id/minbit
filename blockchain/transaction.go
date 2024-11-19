package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"log"
	"math/big"
	"strconv"

	"github.com/mr-tron/base58/base58"
)

type Transaction struct {
	TxID      string
	Sender    string
	Recipent  string
	Amount    int
	Signature string
	Inputs    []UTXO
	Outputs   []UTXO
}

var us = &UTXOSet{
	UTXOs: make(map[string]map[int]UTXO),
}

func (tx *Transaction) IsValid() bool {
	pbKeyAddr, _ := PublickKeyToAddress(tx.Sender)
	if len(tx.Inputs) == 0 || len(tx.Outputs) == 0 || tx.Amount > us.GetTotalUTXOsByAddress(pbKeyAddr) {
		return false
	}

	pubKeyBytes, err := base58.Decode(tx.Sender)
	if err != nil {
		log.Fatal("Invalid public key")
		return false
	}
	sigBytes, err := base58.Decode(tx.Signature)
	if err != nil {
		log.Fatal("Invalid signature")
	}

	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:])

	publicKey := ecdsa.PublicKey{
		Curve: elliptic.P224(),
		X:     new(big.Int).SetBytes(pubKeyBytes[:32]),
		Y:     new(big.Int).SetBytes(pubKeyBytes[32:]),
	}

	hash := tx.Hash()

	if !ecdsa.Verify(&publicKey, hash, r, s) {
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
