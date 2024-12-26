package tui

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shu8h0-null/minimal-btc/blockchain"
)

var (
	winWidth  int
	winHeight int
)

func Centered(m tea.Model, content string, w, h int) string {
	centeredContent := lipgloss.Place(
		w, h,
		lipgloss.Center, lipgloss.Center,
		boxStyle.Render(content),
		lipgloss.WithWhitespaceChars(" "),
	)
	return centeredContent
}

func sendBTC(address string, amount int) error {
	inputs, err := getInputs(blockchain.GetInputsForTxByAddress(UserWallet.Address), amount)
	if err != nil {
		return err
	}
	outputs := calculateOutputs(inputs, amount, address, UserWallet.Address)
	tx := &blockchain.Transaction{
		Recipent:  address,
		Sender:    UserWallet.PublicKey,
		Amount:    amount,
		Signature: "",
		Inputs:    inputs,
		Outputs:   outputs,
	}
	tx.Sign(UserWallet.privateKey)
	return nil
}

func getInputs(inputs []blockchain.Input, amtToBeSent int) ([]blockchain.Input, error) {
	inputAmtSum := 0
	if len(inputs) == 0 {
		return nil, errors.New("Transaction failed not enough balance")
	}
	for _, input := range inputs {
		if inputAmtSum >= amtToBeSent {
			break
		} else {
			inputs = append(inputs, input)
		}
	}

	if inputAmtSum < amtToBeSent {
		return nil, errors.New("Transaction failed not enough balance")
	}
	return inputs, nil
}

func addressValidator(input string) error {
	if UserWallet.Address == input {
		return errors.New("Invalid wallet address: Cannot send to your own address!")
	}
	if len(input) != 58 {
		return errors.New("Invalid wallet address: Address length cannot be less than 58 characters!")
	}
	const base58Chars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	base58Regx := fmt.Sprintf("^[%s]+$", regexp.QuoteMeta(base58Chars))
	re := regexp.MustCompile(base58Regx)
	if !re.MatchString(input) {
		return errors.New("Invalid wallet address: cannot use 0(zero), O(uppercase o), I(uppercase i), l(lowercase L) in the address!")
	}
	return nil
}

func amountValidator(input string) error {
	v, err := strconv.Atoi(input)
	if input == "" || v == 0 || err != nil {
		return errors.New("Invalid amount: Amount should be a valid number and cannot be zero!")
	}
	return nil
}

func calculateOutputs(inputs []blockchain.Input, amount int, recipAddr, senderAddr string) []blockchain.Output {
	totalInputAmount := 0
	var outputs []blockchain.Output
	for _, inp := range inputs {
		totalInputAmount += inp.Value
	}
	if totalInputAmount > amount {
		// we can have two outputs at max since in a single transaction one address can send to only one address
		output1 := blockchain.Output{
			OutputIndex: 0,
			Value:       totalInputAmount - amount,
			Address:     senderAddr,
		}
		output2 := blockchain.Output{
			OutputIndex: 0,
			Value:       amount,
			Address:     recipAddr,
		}
		outputs = append(outputs, output1, output2)
	} else if totalInputAmount == amount {
		output := blockchain.Output{
			OutputIndex: 0,
			Value:       amount,
			Address:     recipAddr,
		}
		outputs = append(outputs, output)
	}
	return outputs
}
