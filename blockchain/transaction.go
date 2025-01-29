package blockchain

import (
	// "context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	// "errors"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
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
		logger.Error("Error generating signature: ", err)
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

	if len(tx.Sender) != 58 {
		logger.Error("Invalid length of sender address")
		return false
	}

	if len(tx.Recipent) != 58 {
		logger.Error("Invalid length of recipent address")
		return false
	}

	if len(tx.Outputs) == 0 {
		logger.Info("Invalid number of output: ", len(tx.Outputs))
		return false
	}

	if tx.Amount > us.GetTotalBalByAddress(tx.Sender) {
		logger.Info("Balance insufficient to process the transaction")
	}

	if !tx.IsCoinbase {
		for _, input := range tx.Inputs {
			correspondingOutput, err := us.getUTXO(input.PrevTxID, input.OutputIndex)
			if err != nil {
				logger.Error("Invalid input, found no corressponding output", err)
			}

			if err := UnlockUTXO(input, correspondingOutput, txIDBytes); err != nil {
				return false
			}
		}

	}

	return true
}

func (tx *Transaction) Hash() []byte {
	txData := tx.TxID + tx.Sender + tx.Recipent + strconv.Itoa(tx.Amount) + tx.Timestamps + strconv.FormatBool(tx.IsCoinbase)

	for _, input := range tx.Inputs {
		txData += input.PrevTxID + strconv.Itoa(input.OutputIndex) + strconv.Itoa(input.Value)
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
	spew.Dump(coinbaseTx)
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

// yet to be implemented...
// func sendTransaction(ctx context.Context, amount int, receipentAddr string, txPublisher *pubsub.Topic) error {
// 	if amount > us.GetTotalBalByAddress(NodeWallet.Address) {
// 		logger.Errorf("Not enough balance!")
// 		return errors.New("Not enough balance!")
// 	}
//
// 	availableInputs := GetAvailableIns(NodeWallet.Address)
// 	inputs, err := ResolveInputs(availableInputs, amount)
// 	if err != nil {
// 		logger.Error("Error resolving inputs:", err)
// 		return err
// 	}
//
// 	outputs := DeriveOutputs(inputs, amount, receipentAddr, NodeWallet.Address)
//
// 	tx := &Transaction{
// 		Amount:     amount,
// 		Sender:     NodeWallet.Address,
// 		Recipent:   receipentAddr,
// 		Inputs:     inputs,
// 		Outputs:    outputs,
// 		Timestamps: time.Now().String(),
// 		IsCoinbase: false,
// 	}
// 	// Fix
// 	tx.Sign(NodeWallet.PrivateKey)
// 	mempool.AddTransaction(tx)
//
// 	return nil
// }
