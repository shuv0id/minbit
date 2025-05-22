package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"

	bolt "go.etcd.io/bbolt"
)

// Input represents input in a transaction
type Input struct {
	PrevTxID    string `json:"prev_tx_id"`
	OutputIndex int    `json:"output_index"`
	ScriptSig   string `json:"script_sig"` // Note: ScriptSig here is a simplified representation and does not reflect the actual Bitcoin implementation.
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
	store *Store
	mu    sync.Mutex
}

func NewUTXOSet(store *Store, utxoBucket string) (*UTXOSet, error) {
	us := &UTXOSet{
		UTXOs: make(map[string]map[int]UTXO),
		store: store,
	}
	err := us.Load()
	if err != nil {
		return nil, err
	}

	return us, nil
}

func (us *UTXOSet) Load() error {
	us.mu.Lock()
	defer us.mu.Unlock()

	if us.store == nil {
		return errors.New("Cannot load utxoSet. UTXOSet is not initialised with a Db")
	}

	err := us.store.Db().View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(us.store.UTXOSetBucket()))
		if b == nil {
			return errors.New("bucket not found")
		}

		err := b.ForEach(func(k, v []byte) error {
			u, err := deserializeUTXO(v)
			us.addUTXO(u.TxID, u.OutputIndex, u.Value, u.ScriptPubKey)
			return err
		})

		return err
	})

	return err
}

func (us *UTXOSet) addUTXO(txID string, outputIndex int, value int, scriptPubKey string) {
	us.mu.Lock()
	defer us.mu.Unlock()

	if _, exists := us.UTXOs[txID]; !exists {
		us.UTXOs[txID] = make(map[int]UTXO)
	}
	us.UTXOs[txID][outputIndex] = UTXO{
		TxID:         txID,
		OutputIndex:  outputIndex,
		Value:        value,
		ScriptPubKey: scriptPubKey,
	}
}

func (us *UTXOSet) removeUTXO(txID string, outputIndex int) {
	us.mu.Lock()
	defer us.mu.Unlock()

	if utxos, exists := us.UTXOs[txID]; exists {
		delete(utxos, outputIndex)
		if len(utxos) == 0 {
			delete(us.UTXOs, txID)
		}
	}
}

// Update applies a batch of transactions to the UTXO set, writing updates to the database with retries.
// It removes spent UTXOs and adds new ones based on transaction outputs
func (us *UTXOSet) Update(txs []Transaction) error {
	for _, transaction := range txs {
		err := RetryN(func() error {
			err := us.WriteUpdate(transaction)
			return err
		}, 3, fmt.Sprintf("Failed to update utxoSet in db for transaction:[%s]", transaction.TxID))

		if err != nil {
			return err
		}
	}

	for _, tx := range txs {
		for _, input := range tx.Inputs {
			us.removeUTXO(input.PrevTxID, input.OutputIndex)
		}
		for index, output := range tx.Outputs {
			us.addUTXO(tx.TxID, index, output.Value, output.ScriptPubKey)
		}
	}

	return nil
}

// WriteUpdate updates the UTXO set by deleting spent outputs and adding new ones.
// Returns an error if any database operation fails.
func (us *UTXOSet) WriteUpdate(transaction Transaction) error {
	err := us.store.Db().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(us.store.UTXOSetBucket()))
		for _, input := range transaction.Inputs {
			k := input.PrevTxID + "_" + strconv.Itoa(input.OutputIndex)
			if err := b.Delete([]byte(k)); err != nil {
				return err
			}
		}

		for index, output := range transaction.Outputs {
			var v bytes.Buffer
			k := transaction.TxID + "_" + strconv.Itoa(index)
			gob.NewEncoder(&v).Encode(output)
			if err := b.Put([]byte(k), v.Bytes()); err != nil {
				return err
			}
		}
		return nil
	})

	return err
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
				log.Error("Error getting balance: Cannot decode hex encoded scriptPubKey", err)
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
				log.Error("Error decoding scriptPubKey", err)
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
func (us *UTXOSet) DeriveOutputs(inputs []Input, amount int, recipAddr, senderAddr string) []Output {
	totalInputAmount := 0
	var outputs []Output
	for _, inp := range inputs {
		utxo, err := us.GetUTXO(inp.PrevTxID, inp.OutputIndex)

		if err != nil {
			log.Error("Invalid input, found no corressponding output", err)
			return outputs
		}

		totalInputAmount += utxo.Value
	}

	senderPubKeyHash, err := AddressToPubKeyHash(senderAddr)
	if err != nil {
		log.Error("Invalid sender address")
		return outputs
	}
	recipAddrPubKeyHash, err := AddressToPubKeyHash(recipAddr)
	if err != nil {
		log.Error("Invalid recipent address")
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
			ScriptPubKey: hex.EncodeToString(recipAddrPubKeyHash),
		}
		outputs = append(outputs, output)
	}
	return outputs
}

func UnlockUTXO(input Input, utxo UTXO, txHash []byte) error {
	pubKeyHashHex := utxo.ScriptPubKey

	scriptSigBytes, err := hex.DecodeString(input.ScriptSig)
	if err != nil {
		return fmt.Errorf("Error decoding scriptsig hex of input", err)
	}

	sigBytes, pubkeyBytes, err := SplitScriptSig(scriptSigBytes)
	if err != nil {
		return fmt.Errorf("Invalid scriptsig of input", err)
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), pubkeyBytes)
	publicKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	pubKeyHashBytes, err := PublicKeyToPubKeyHash(publicKey)
	if err != nil {
		return err
	}
	if pubKeyHashHex != hex.EncodeToString(pubKeyHashBytes) {
		return errors.New("Cannot spend the corressponding output: Owner mismatched")
	}

	verified := ecdsa.VerifyASN1(publicKey, txHash, sigBytes)
	if !verified {
		return errors.New("Invalid signature")
	}

	return nil
}

func serializeUTXO(u UTXO) ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(u)
	return buf.Bytes(), err
}

func deserializeUTXO(data []byte) (UTXO, error) {
	var u UTXO
	err := gob.NewDecoder(bytes.NewReader(data)).Decode(&u)
	return u, err
}
