package blockchain

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
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

func (tx *Transaction) isValid() bool {
	txIDBytes, err := hex.DecodeString(tx.TxID)
	if err != nil {
		logger.Error("Invalid transaction id", err)
		return false
	}

	_, err = base58.Decode(tx.Sender)
	if err != nil {
		logger.Error("Invalid Sender Address", err)
		return false
	}
	_, err = base58.Decode(tx.Recipent)
	if err != nil {
		logger.Error("Invalid Recipent Address", err)
		return false
	}

	if len(tx.Outputs) == 0 {
		logger.Info("Invalid number of output: ", len(tx.Outputs))
		return false
	}

	if tx.Amount > utxoSet.GetTotalBalByAddress(tx.Sender) {
		logger.Info("Balance insufficient to process the transaction")
	}

	if !tx.IsCoinbase {
		for _, input := range tx.Inputs {
			utxo, err := utxoSet.GetUTXO(input.PrevTxID, input.OutputIndex)
			if err != nil {
				logger.Error("Invalid input, found no corressponding output", err)
			}

			if err := UnlockUTXO(input, utxo, txIDBytes); err != nil {
				return false
			}
		}

	}

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

func GenerateCoinbaseTx() Transaction {
	coinbaseReward := 6 // hardcoded block reward amount value; block reward halving not implemented
	var outputs []Output

	WalletPubKeyHash, err := AddressToPubKeyHash(NodeWallet.Address)
	if err != nil {
		logger.Error("Invalid Wallet address")
		return Transaction{}
	}

	rewardOutput := Output{
		Value:        coinbaseReward,
		ScriptPubKey: hex.EncodeToString(WalletPubKeyHash),
	}

	outputs = append(outputs, rewardOutput)
	coinbaseTx := Transaction{
		Amount:     coinbaseReward,
		Recipent:   NodeWallet.Address,
		IsCoinbase: true,
		Outputs:    outputs,
		Timestamps: time.Now().String(),
	}
	coinbaseTx.TxID = hex.EncodeToString(coinbaseTx.Hash())
	return coinbaseTx
}

func TransactionsToString(transactions []Transaction) string {
	txStr := []string{}
	for _, t := range transactions {
		inputsStr := []string{}
		for _, input := range t.Inputs {
			inputsStr = append(inputsStr, input.PrevTxID+", "+strconv.Itoa(input.OutputIndex)+", ")
		}

		outputsStr := []string{}
		for _, output := range t.Outputs {
			outputsStr = append(outputsStr, strconv.Itoa(output.Value)+", "+output.ScriptPubKey)
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

func sendTransaction(ctx context.Context, receipAddr string, amount int, txPublisher *pubsub.Topic) error {
	_, err := base58.Decode(receipAddr)
	if err != nil {
		logger.Errorf("Invalid receipAddr %v", err)
		return err
	}

	if receipAddr == NodeWallet.Address {
		logger.Errorf("Address of sender and recipent cannot be equal")
		return fmt.Errorf("Address of sender and recipent cannot be equal")
	}

	if amount > utxoSet.GetTotalBalByAddress(NodeWallet.Address) {
		logger.Errorf("Not enough balance!")
		return fmt.Errorf("Not enough balance!")
	}

	utxos := utxoSet.GetAvailableUTXOS(NodeWallet.Address)

	inputs, err := ResolveInputs(utxos, amount)
	if err != nil {
		return err
	}

	outputs := utxoSet.DeriveOutputs(inputs, amount, receipAddr, NodeWallet.Address)

	tx := Transaction{
		Sender:     NodeWallet.Address,
		Recipent:   receipAddr,
		Amount:     amount,
		Inputs:     inputs,
		Outputs:    outputs,
		Timestamps: time.Now().String(),
		IsCoinbase: false,
	}

	pubKey, err := NodeWallet.PublicKey.ECDH()
	if err != nil {
		logger.Error("Error converting ecdsa public key to ecdh public key")
		return fmt.Errorf("Error converting ecdsa public key to ecdh public key %v", err)
	}

	pubKeyBytes := pubKey.Bytes()

	txHash := tx.Hash()
	tx.TxID = hex.EncodeToString(txHash)

	sigBytes, err := tx.Sign(NodeWallet.PrivateKey)

	if err != nil {
		logger.Error("Error generating signature: ", err)
		return err
	}

	scriptSig := CreateScriptSig(sigBytes, pubKeyBytes)

	for _, input := range tx.Inputs {
		input.ScriptSig = hex.EncodeToString(scriptSig)
	}

	mempool.AddTransaction(&tx)

	if txPublisher.String() != topicTx {
		return fmt.Errorf("wrong topic for sending transaction. Expected: %s, got: %s", topicTx, txPublisher.String())
	}

	txBytes, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %w", err)
	}

	err = txPublisher.Publish(ctx, txBytes)
	if err != nil {
		return fmt.Errorf("failed to publish transaction: %w", err)
	}

	return nil
}
