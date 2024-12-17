package blockchain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"os"

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

func checkPortAvailability(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
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
