package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"

	"github.com/mr-tron/base58/base58"
)

type Transaction struct {
	TxID      string   `json:"trasaction_id"`
	Sender    string   `json:"sender"`
	Recipent  string   `json:"recipent"`
	Amount    int      `json:"amount"`
	Signature string   `json:"signature"`
	Inputs    []Input  `json:"inputs"`
	Outputs   []Output `json:"outputs"`
}

func (tx *Transaction) Sign(privateKey *ecdsa.PrivateKey) error {
	hash := tx.Hash()
	tx.TxID = hex.EncodeToString(hash)

	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, hash)
	if err != nil {
		logger.Error("Error generating signature: ", err)
		return err
	}

	tx.Signature = hex.EncodeToString(signature)

	return nil
}

func (tx *Transaction) isValid() bool {
	pbuKeyAddr, _ := PublicKeyHexToAddress(tx.Sender)
	if len(tx.Inputs) == 0 || len(tx.Outputs) == 0 || tx.Amount > us.GetTotalUTXOsByAddress(pbuKeyAddr) {
		return false
	}

	sigBytes, err := base58.Decode(tx.Signature)
	if err != nil {
		logger.Error("Invalid signature")
	}

	pubKeyBytes, err := hex.DecodeString(pbuKeyAddr)
	if err != nil {
		logger.Error("Invalid public key")
		return false
	}
	if len(pubKeyBytes) != 64 {
		logger.Info("Invalid publick key length")
		return false
	}

	publicKey := ecdsa.PublicKey{
		Curve: elliptic.P256(),
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
		txData += strconv.Itoa(input.OutputIndex) + input.PrevTxID
	}

	for _, output := range tx.Outputs {
		txData += strconv.Itoa(output.Value) + strconv.Itoa(output.OutputIndex) + output.Address
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
