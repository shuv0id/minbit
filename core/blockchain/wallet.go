package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	Address    string
}

// NewWallet generates a new wallet.
func NewWallet() (*Wallet, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("Error generating private key for wallet: %v", err)
	}

	wallet, err := ConstructWallet(privateKey)
	if err != nil {
		return nil, err
	}
	return wallet, nil
}

// ConstructWallet contructs an Wallet from a given *ecdsa.PrivateKey
func ConstructWallet(privKey *ecdsa.PrivateKey) (*Wallet, error) {
	publicKey := &privKey.PublicKey
	pubKeyHash, err := PublicKeyToPubKeyHash(publicKey)

	if err != nil {
		return nil, err
	}
	address := PubKeyHashToAddress(pubKeyHash)

	return &Wallet{
		PrivateKey: privKey,
		PublicKey:  publicKey,
		Address:    address,
	}, nil
}

// Save saves the wallet in the provided dir
func (wallet *Wallet) Save(dir string) error {
	var walletBytes bytes.Buffer
	gob.Register(elliptic.P256())

	gob.NewEncoder(&walletBytes).Encode(wallet)

	file := filepath.Join(dir, "wallet.dat")
	if err := os.WriteFile(file, walletBytes.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to write key: %v", err)
	}
	return nil
}

// LoadWallet load wallet from the specified file
func LoadWallet(path string) (*Wallet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Error reading wallet file", err)
	}

	var wallet Wallet
	err = gob.NewDecoder(bytes.NewReader(data)).Decode(wallet)
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}
