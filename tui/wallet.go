// This wallet code is specific to the end user wallets; not for nodes
package tui

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"

	"github.com/shu8h0-null/minimal-btc/blockchain"
)

var wall Wallet

type Wallet struct {
	privateKey *ecdsa.PrivateKey
	PublicKey  string
	Address    string
}

func GenerateWallet() *Wallet {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return &Wallet{}
	}

	publicKey := append(privateKey.X.Bytes(), privateKey.PublicKey.Y.Bytes()...)
	publicKeyHex := hex.EncodeToString(publicKey)

	address, err := blockchain.PublicKeyHexToAddress(publicKeyHex)
	if err != nil {
		return &Wallet{}
	}
	return &Wallet{
		privateKey: privateKey,
		PublicKey:  publicKeyHex,
		Address:    address,
	}
}
