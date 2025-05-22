package netstack

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// GeneratePrivKeyForNode is a helper function to create private key for nodes
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

// CheckPortAvailability checks if a port is available to listen to
func CheckPortAvailability(host string, port int) bool {
	address := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// LoadNodePrivKey loads a private key from ./keys/<id>/node.key.
func LoadNodePrivKey(id string) (crypto.PrivKey, error) {
	key, err := os.ReadFile(filepath.Join("keys", id, "node.key"))
	if err != nil {
		return nil, fmt.Errorf("failed to read key: %v", err)
	}
	priv, err := crypto.UnmarshalPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal key: %v", err)
	}
	return priv, nil
}

// SaveHostPrivKey saves the private key to ./keys/<peer.ID>/node.key.
// Creates the directory if not already present
func SaveHostPrivKey(id peer.ID, priv crypto.PrivKey) error {
	dir := filepath.Join("keys", id.String())
	file := filepath.Join(dir, "node.key")

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	privBytes, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal key: %v", err)
	}

	if err := os.WriteFile(file, privBytes, 0600); err != nil {
		return fmt.Errorf("failed to write key: %v", err)
	}
	return nil
}

// SaveHostAddrToFile stores the full multiaddress of the host in a `onlinePeers` file.
// Creates or updates the file with the peer ID as key and full multiaddress as value.
func SaveHostAddrToFile(h host.Host) error {
	peerID := h.ID().String()
	p2pAddr, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", peerID))
	if err != nil {
		return fmt.Errorf("Error forming p2pAddr string: %v", err)
	}

	fullAddr := h.Addrs()[0].Encapsulate(p2pAddr).String()

	file, err := os.OpenFile(onlinePeers, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("Failed to open file %s: %v\n", onlinePeers, err)
	}
	defer file.Close()

	var peers OnlinePeers
	err = json.NewDecoder(file).Decode(&peers)
	if err != nil {
		if err == io.EOF {
			peers = make(OnlinePeers)
		} else {
			return fmt.Errorf("Failed to decode json: %v\n", err)
		}
	}

	peers[peerID] = fullAddr

	file.Truncate(0)
	file.Seek(0, 0)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	err = encoder.Encode(peers)
	if err != nil {
		return fmt.Errorf("Failed to write to file: %v\n", err)
	}

	return nil
}

// RemoveHostMultiaddrFromFile removes the multiaddress entry of a specific peer ID from the `onlinePeers` file
func RemoveHostMultiaddrFromFile(peerID peer.ID) error {
	peerid := peerID.String()
	file, err := os.OpenFile(onlinePeers, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("Failed to open file %s: %v\n", onlinePeers, err)
	}
	defer file.Close()

	var peers OnlinePeers
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&peers)
	if err != nil {
		if err == io.EOF {
			peers = make(OnlinePeers)
		} else {
			return fmt.Errorf("Failed to decode JSON: %v\n", err)
		}
	}

	if _, exists := peers[peerid]; !exists {
		return nil
	}

	file.Truncate(0)
	file.Seek(0, 0)

	delete(peers, peerid)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	err = encoder.Encode(peers)
	if err != nil {
		return fmt.Errorf("Failed to write to file: %v\n", err)
	}

	return nil
}

// ExtractPeerID takes a libp2p multiaddress string and returns the peer ID
func ExtractPeerID(addrStr string) (peer.ID, error) {
	addr, err := ma.NewMultiaddr(addrStr)
	if err != nil {
		return "", fmt.Errorf("invalid multiaddress: %w", err)
	}

	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return "", fmt.Errorf("failed to extract peer info: %w", err)
	}

	return info.ID, nil
}
