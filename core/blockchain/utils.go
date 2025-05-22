package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/mr-tron/base58/base58"
	"golang.org/x/crypto/ripemd160"
)

// PublicKeyToPubKeyHash convert the public key pubkey to public key hash by first hashing the public key with
// sha256Hash and then the resulting hash is again hashed with ripemd160 hash
func PublicKeyToPubKeyHash(pubKey *ecdsa.PublicKey) ([]byte, error) {
	pubkeyECDH, err := pubKey.ECDH()
	if err != nil {
		return nil, fmt.Errorf("Invalid public key: %v", err)
	}
	publicKeyBytes := pubkeyECDH.Bytes()

	sha256Hash := sha256.Sum256(publicKeyBytes)
	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hash[:])
	ripemd160Hash := ripemd160Hasher.Sum(nil)

	return ripemd160Hash, nil
}

// PubKeyHashToAddress convert the public key bytes to address in base58-encoded string
func PubKeyHashToAddress(pubKeyHash []byte) string {
	firstHash := sha256.Sum256(pubKeyHash)
	secondHash := sha256.Sum256(firstHash[:])

	checksum := secondHash[:4]

	addressBytes := append(pubKeyHash, checksum...)
	return base58.Encode(addressBytes)
}

// AddressToPubKeyHash convert the base58-encoded string to public key hash
func AddressToPubKeyHash(address string) ([]byte, error) {
	addressBytes, err := base58.Decode(address)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode address", err)
	}

	pubKeyHash := addressBytes[:len(addressBytes)-4] // [1, 2, 3, 4, 5, 6, 7, 8, 9]
	firstHash := sha256.Sum256(pubKeyHash)
	secondHash := sha256.Sum256(firstHash[:])

	checksumbytes := addressBytes[len(addressBytes)-4:]

	if !bytes.Equal(checksumbytes, secondHash[:4]) {
		return nil, fmt.Errorf("Checksum mismatched ", checksumbytes, secondHash[:4])
	}
	return pubKeyHash, nil
}

func CreateScriptSig(signature, pubKey []byte) []byte {
	var scriptSig bytes.Buffer

	scriptSig.WriteByte(byte(len(signature)))
	scriptSig.Write(signature)

	scriptSig.WriteByte(byte(len(pubKey)))
	scriptSig.Write(pubKey)

	return scriptSig.Bytes()
}

// SplitScriptSig splits the scriptSig byte slice into its consitituent signature bytes and public key bytes
func SplitScriptSig(scriptSig []byte) ([]byte, []byte, error) { // Ensure the scriptSig has enough data to process
	if len(scriptSig) < 2 {
		return nil, nil, errors.New("scriptSig too short to contain both signature and public key")
	}

	sigLength := int(scriptSig[0])
	sigBytes := scriptSig[1 : 1+sigLength]

	pubKeyLength := int(scriptSig[1+sigLength])
	pubKeyBytes := scriptSig[1+sigLength+1 : 1+sigLength+1+pubKeyLength]

	return sigBytes, pubKeyBytes, nil
}

// RetryN retries the given function up to n times if it returns an error.
// Logs retryMsg before each retry (except the last).
// Returns nil whether it succeeds or not.
func RetryN(fn func() error, n int, retryMsg string) error {
	for i := 1; i <= n; i++ {
		err := fn()
		if err == nil {
			break
		}
		if i < n {
			fmt.Printf("%s (attempt %d/%d)", retryMsg, i, n)
		}
	}
	return nil
}
