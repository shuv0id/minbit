// This wallet code is specifically coupled with the node itself
package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
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

	address, err := PublicKeyHexToAddress(publicKeyHex)
	if err != nil {
		return &Wallet{}
	}
	return &Wallet{
		privateKey: privateKey,
		PublicKey:  publicKeyHex,
		Address:    address,
	}
}
