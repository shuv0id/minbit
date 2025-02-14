package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/peer"
)

var NodeWallet *Wallet

type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	Address    string
}

// NewWallet generates a new wallet with a new private key.
// New key it is saved to a PEM file under ./keys/<id>/wallet.pem.
func NewWallet() *Wallet {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		logger.Error("Error generating private key for wallet: %v", err)
		return &Wallet{}
	}
	wallet := ConstructWallet(privateKey)
	SaveWalletPrivKey(node.ID(), privateKey)
	return wallet
}

// ConstructWallet contructs an Wallet from a given *ecdsa.PrivateKey
func ConstructWallet(privKey *ecdsa.PrivateKey) *Wallet {
	publicKey := &privKey.PublicKey
	pubKeyHash := PublicKeyToPubKeyHash(publicKey)
	address := PubKeyHashToAddress(pubKeyHash)

	return &Wallet{
		PrivateKey: privKey,
		PublicKey:  publicKey,
		Address:    address,
	}
}

// LoadWalletPrivKey loads an ECDSA private key from ./keys/<id>/wallet.pem.
// Expects the key to be in PEM format with type "EC PRIVATE KEY".
func LoadWalletPrivKey(id string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(filepath.Join("./keys", id, "wallet.pem"))
	if err != nil {
		return nil, fmt.Errorf("failed to read key: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, errors.New("invalid PEM block type")
	}

	privKey, err := x509.ParseECPrivateKey(block.Bytes)
	return privKey, nil
}

// SaveWalletPrivKey saves an ECDSA private key in PEM format to ./keys/<peer.ID>/wallet.pem.
// Creates the directory if it doesn't exist.
func SaveWalletPrivKey(id peer.ID, priv *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}

	data := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	}
	dataBytes := pem.EncodeToMemory(data)

	dir := filepath.Join("./keys", id.String())
	file := filepath.Join(dir, "wallet.pem")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if err := os.WriteFile(file, dataBytes, 0600); err != nil {
		return fmt.Errorf("failed to write key: %v", err)
	}
	return nil
}
