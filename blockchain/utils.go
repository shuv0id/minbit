package blockchain

import (
	"crypto/rand"
	"crypto/sha256"
	"io"
	mrand "math/rand"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mr-tron/base58/base58"
	"golang.org/x/crypto/ripemd160"
)

// PublickKeyToAddress convert the base58-encoded public key pubkey to address
func PublickKeyToAddress(pubKey string) (string, error) {
	pubKeyBytes, err := base58.Decode(pubKey)
	if err != nil {
		logger.Error("Invalid public key")
		return "", err
	}
	sha256Hash := sha256.Sum256(pubKeyBytes[:])

	ripemdHash := ripemd160.New().Sum(sha256Hash[:])

	return base58.Encode(ripemdHash), nil
}

func GeneratePrivKeyForNode(randseed int64) (crypto.PrivKey, error) {
	var r io.Reader
	if randseed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(randseed))
	}

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}
	return priv, err
}
