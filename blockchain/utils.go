package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/mr-tron/base58/base58"
	"golang.org/x/crypto/ripemd160"
)

// PublicKeyToPubKeyHash convert the public key pubkey to public key hash by first hashing the public key with
// sha256Hash and then the resulting hash is again hashed with ripemd160 hash
func PublicKeyToPubKeyHash(pubKey *ecdsa.PublicKey) []byte {
	pubkeyECDH, err := pubKey.ECDH()
	if err != nil {
		logger.Error("Invalid public key")
		return nil
	}
	publicKeyBytes := pubkeyECDH.Bytes()

	sha256Hash := sha256.Sum256([]byte(publicKeyBytes))
	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hash[:])
	ripemd160Hash := ripemd160Hasher.Sum(nil)

	return ripemd160Hash
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
		logger.Error("Failed to decode address", err)
		return nil, err
	}

	pubKeyHash := addressBytes[:len(addressBytes)-4]
	firstHash := sha256.Sum256(pubKeyHash)
	secondHash := sha256.Sum256(firstHash[:])

	checksumbytes := addressBytes[len(addressBytes)-4:]

	if !bytes.Equal(checksumbytes, secondHash[:4]) {
		logger.Error("Checksum mismatched ", checksumbytes, secondHash[:4])
		return nil, err
	}
	return pubKeyHash, nil
}

func UnlockUTXO(input Input, utxo UTXO, txHash []byte) error {
	pubKeyHashBytes := utxo.ScriptPubKey

	scriptSigBytes, err := hex.DecodeString(input.ScriptSig)
	if err != nil {
		logger.Error("Error decoding scriptsig hex of input", err)
		return err
	}

	sigBytes, pubkeyBytes, err := splitScriptSig(scriptSigBytes)
	if err != nil {
		logger.Error("Invalid scriptsig of input", err)
		return err
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), pubkeyBytes)
	publicKey := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	if pubKeyHashBytes != hex.EncodeToString(PublicKeyToPubKeyHash(publicKey)) {
		logger.Error("Cannot spend the corressponding output: Owner mismatched")
		return err
	}

	verified := ecdsa.VerifyASN1(publicKey, txHash, sigBytes)
	if !verified {
		logger.Error("Invalid signature")
		return errors.New("Invalid signature")
	}

	return nil
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

func CheckPortAvailability(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

func CreateScriptSig(signature, pubKey []byte) []byte {
	var scriptSig bytes.Buffer

	scriptSig.WriteByte(byte(len(signature)))
	scriptSig.Write(signature)

	scriptSig.WriteByte(byte(len(pubKey)))
	scriptSig.Write(pubKey)

	return scriptSig.Bytes()
}

// splitScriptSig splits the scriptSig byte slice into its consitituent signature bytes and public key bytes
func splitScriptSig(scriptSig []byte) ([]byte, []byte, error) { // Ensure the scriptSig has enough data to process
	if len(scriptSig) < 2 {
		return nil, nil, errors.New("scriptSig too short to contain both signature and public key")
	}

	sigLength := int(scriptSig[0])
	sigBytes := scriptSig[1 : 1+sigLength]

	pubKeyLength := int(scriptSig[1+sigLength])
	pubKeyBytes := scriptSig[1+sigLength+1 : 1+sigLength+1+pubKeyLength]

	return sigBytes, pubKeyBytes, nil
}

func writeNodeAddrToJSONFile(addr string, peerID string, filename string) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		logger.Errorf("Failed to open file %s: %v\n", filename, err)
		return err
	}

	nodeInfo := NodeIdentifier{
		PeerID: peerID,
		Addr:   addr,
	}

	var nodes []NodeIdentifier
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&nodes)
	if err != nil && err.Error() != "EOF" {
		logger.Errorf("Failed to decode json: %v\n", err)
		return err
	}

	nodes = append(nodes, nodeInfo)
	file.Truncate(0)
	file.Seek(0, 0)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	err = encoder.Encode(nodes)
	if err != nil {
		logger.Errorf("Failed to write to file: %v\n", err)
		return err
	}

	return nil
}

func removeNodeInfoFromJSONFile(peerID string, filename string) error {
	file, err := os.OpenFile(filename, os.O_RDWR, 0666)
	if err != nil {
		logger.Errorf("Failed to open file %s: %v\n", filename, err)
		return err
	}

	var nodes []NodeIdentifier
	var nodeFound bool
	decoder := json.NewDecoder(file)
	decoder.Decode(&nodes)

	for i, n := range nodes {
		if n.PeerID == peerID {
			nodes = append(nodes[:i], nodes[i+1:]...)
			nodeFound = true
			break
		}
	}

	if nodeFound {
		file.Truncate(0)
		file.Seek(0, 0)

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", " ")
		err = encoder.Encode(nodes)
		if err != nil {
			logger.Errorf("Failed to write to file: %v\n", err)
			return err
		}
	}

	return nil
}
