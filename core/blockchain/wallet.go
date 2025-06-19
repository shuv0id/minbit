package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/shu8h0-null/minbit/core/config"
)

type Wallet struct {
	Id         string
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	Address    string
}

type serializableKey struct {
	D, X, Y *big.Int
}

type serializableWallet struct {
	Id      string
	Key     serializableKey
	Address string
}

// NewWallet generates a new wallet.
func NewWallet(id string) (*Wallet, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("Error generating private key for wallet: %v", err)
	}

	wallet, err := ConstructWallet(id, privateKey)
	if err != nil {
		return nil, err
	}

	if err := wallet.Save(); err != nil {
		return nil, fmt.Errorf("failed to save wallet: %w", err)
	}

	return wallet, nil
}

// ConstructWallet contructs an Wallet from a given *ecdsa.PrivateKey
func ConstructWallet(id string, privKey *ecdsa.PrivateKey) (*Wallet, error) {
	publicKey := &privKey.PublicKey
	pubKeyHash, err := PublicKeyToPubKeyHash(publicKey)

	if err != nil {
		return nil, err
	}
	address := PubKeyHashToAddress(pubKeyHash)

	return &Wallet{
		Id:         id,
		PrivateKey: privKey,
		PublicKey:  publicKey,
		Address:    address,
	}, nil
}

func (wallet *Wallet) Save() error {
	if wallet == nil {
		return errors.New("wallet not initialized")
	}

	walletDir := filepath.Join(config.WalletDir(), wallet.Id)
	walletFile := filepath.Join(walletDir, "wallet.dat")
	if err := os.MkdirAll(walletDir, 0700); err != nil {
		return err
	}

	key := serializableKey{
		D: wallet.PrivateKey.D,
		X: wallet.PrivateKey.PublicKey.X,
		Y: wallet.PrivateKey.PublicKey.Y,
	}

	sw := serializableWallet{
		Id:      wallet.Id,
		Key:     key,
		Address: wallet.Address,
	}

	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(sw)
	if err != nil {
		return err
	}

	return os.WriteFile(walletFile, buf.Bytes(), 0600)
}

// LoadWallet load wallet with the given id
func LoadWallet(id string) (*Wallet, error) {
	path := filepath.Join(config.WalletDir(), id, "wallet.dat")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var encoded serializableWallet
	err = gob.NewDecoder(bytes.NewReader(data)).Decode(&encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode wallet: %w", err)
	}

	curve := elliptic.P256()

	priv := &ecdsa.PrivateKey{
		D: encoded.Key.D,
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
			X:     encoded.Key.X,
			Y:     encoded.Key.Y,
		},
	}

	pub := &priv.PublicKey

	return &Wallet{
		Id:         encoded.Id,
		PrivateKey: priv,
		PublicKey:  pub,
		Address:    encoded.Address,
	}, nil
}
