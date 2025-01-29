package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

var NodeWallet *Wallet

type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	Address    string
}

func GenerateWallet() *Wallet {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return &Wallet{}
	}

	publicKey := &privateKey.PublicKey

	pubKeyHash := PublicKeyToPubKeyHash(publicKey)
	address := PubKeyHashToAddress(pubKeyHash)

	return &Wallet{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    address,
	}
}
