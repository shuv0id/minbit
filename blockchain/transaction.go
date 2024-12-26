package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	"github.com/mr-tron/base58/base58"
)

type Transaction struct {
	TxID       string   `json:"transaction_id"`
	Sender     string   `json:"sender"`
	Recipent   string   `json:"recipent"`
	Amount     int      `json:"amount"`
	Signature  string   `json:"signature"`
	Inputs     []Input  `json:"inputs"`
	Outputs    []Output `json:"outputs"`
	IsCoinbase bool     `json:"is_coinbase"`
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

func GenerateCoinbaseTx() Transaction {
	coinbaseTx := Transaction{
		Amount:     6,
		Recipent:   wall.Address,
		IsCoinbase: true,
	}
	coinbaseTx.TxID = hex.EncodeToString(coinbaseTx.Hash())
	return coinbaseTx
}

func TransactionsToString(transactions []Transaction) string {
	txStr := []string{}
	for _, t := range transactions {
		inputsStr := []string{}
		for _, input := range t.Inputs {
			inputsStr = append(inputsStr, input.PrevTxID+", "+strconv.Itoa(input.OutputIndex)+", "+strconv.Itoa(input.Value))
		}

		outputsStr := []string{}
		for _, output := range t.Outputs {
			outputsStr = append(outputsStr, strconv.Itoa(output.OutputIndex)+", "+strconv.Itoa(output.Value)+", "+output.Address)
		}

		txStr = append(txStr, strconv.FormatBool(t.IsCoinbase)+", "+strings.Join(inputsStr, ", ")+
			", "+strings.Join(outputsStr, ", "))
	}

	return strings.Join(txStr, "\n")
}

func GetUserTxHistory(walletAddress string) []Transaction {
	var txHistory []Transaction
	for _, b := range bc.Chain {
		for _, tx := range b.TxData {
			if tx.Sender == walletAddress || tx.Recipent == walletAddress {
				txHistory = append(txHistory, b.TxData[1])
			}
		}
	}
	return txHistory
}
