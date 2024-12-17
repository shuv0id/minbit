package blockchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var node host.Host

const nodesAddrFile = "nodes.json"

type NodeIdentifier struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"address"`
}

func StartNode(port int, randseed int64, connectAddr string) error {
	if port == 0 {
		return errors.New("Invalid port!")
	}

	priv, err := GeneratePrivKeyForNode(randseed)
	if err != nil {
		return err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)),
		libp2p.Identity(priv),
	}

	node, err = libp2p.New(opts...)
	if err != nil {
		return err
	}

	// subscribing
	nodePS, err := applyPubSub(node)
	if err != nil {
		logger.Errorf("Error setting up instance for pubsub: %v", err)
		return err
	}

	nodePS.RegisterTopicValidator("block", bTopicValidator)
	nodePS.RegisterTopicValidator("transaction", txTopicValidator)

	bTopic, err := nodePS.Join("block")
	if err != nil {
		logger.Errorf("Error subscribing to topic 'block': %v", err)
		return err
	}
	bSub, _ := bTopic.Subscribe()

	txTopic, err := nodePS.Join("transaction")
	if err != nil {
		logger.Errorf("Error subscribing to topic 'transaction': %s\n", err)
		return err
	}
	txSub, _ := txTopic.Subscribe()

	blockReceiver := make(chan *Block, 1)
	go bReader(bSub, blockReceiver)
	go txReader(txSub)

	node.Network().Notify(&MyNotifiee{})

	fullAddr := node.Addrs()[0].String() + "/p2p/" + node.ID().String()
	logger.Successf("Node started at address: %s\n", fullAddr)

	connectToPeers(node, connectAddr)
	writeNodeAddrToJSONFile(fullAddr, node.ID().String(), nodesAddrFile)
	handleSyncRequests(node)
	go MineBlocks(blockReceiver, bTopic)

	// Signal handling for proper shutdown
	done := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM)

	go func() {
		logger.Info("Received signal: ", <-sigs)
		if err := removeNodeInfoFromJSONFile(node.ID().String(), nodesAddrFile); err != nil {
			logger.Error("Error removing the peer address from json file:", err)
		}
		close(done)
	}()

	select {
	case <-done:
		logger.Error("Node shutting down...")
		node.Close()
		return nil
	}
}

func applyPubSub(h host.Host) (*pubsub.PubSub, error) {
	psOpts := []pubsub.Option{
		pubsub.WithMessageSigning(true),
	}

	return pubsub.NewGossipSub(context.Background(), h, psOpts...)
}

func bTopicValidator(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) == 0 {
		logger.Error("Invalid message: empty data")
		return false
	}

	b := &Block{}

	if err := json.Unmarshal(msg.Data, b); err != nil {
		logger.Error("Invalid json format")
		return false
	}

	return true
}

func txTopicValidator(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) == 0 {
		logger.Error("Invalid message: empty data")
		return false
	}

	tx := &Transaction{}
	if err := json.Unmarshal(msg.Data, tx); err != nil {
		logger.Error("Invalid json format")
		return false
	}

	if !tx.isValid() {
		return false
	}

	return true
}

func bReader(bSub *pubsub.Subscription, bReceiver chan<- *Block) {
	for {
		bMsg, err := bSub.Next(context.Background())
		if err != nil {
			logger.Error("Error receiving next message: ")
		}

		if bMsg.GetFrom() == node.ID() {
			continue
		}
		b := &Block{}

		json.Unmarshal(bMsg.Data, &b)
		logger.Infof("Received block with id: %s from: %s\n", b.Hash, bMsg.GetFrom())
		fmt.Println(Yellow)
		spew.Dump(b)
		fmt.Println(Reset)

		if !b.isValid() {
			continue
		}

		// Signals miner with receiving block
		bReceiver <- b

		bc.AddBlock(b)

		fmt.Println(Yellow, "recevied block added")
		spew.Dump(bc.Chain)
		fmt.Println(Reset)

		mempool.RemoveTransaction(b.TxData.TxID)

	}
}

func txReader(txSub *pubsub.Subscription) {
	for {
		txMsg, _ := txSub.Next(context.Background())
		if txMsg.GetFrom() == node.ID() {
			continue
		}
		tx := &Transaction{}

		json.Unmarshal(txMsg.Data, &tx)
		logger.Infof("Received transaction with id: %s from %s\n", tx.TxID, txMsg.GetFrom())
		mempool.AddTransaction(tx)
	}
}

// connectToPeers establishes a connection between the host and a peer node.
// If `connectAddr` is provided, it directly connects to that address.
// Otherwise, it reads peer addresses from `nodesAddrFile` and randomly selects one to establish the connection.
// Returns an error on failure
func connectToPeers(h host.Host, connectAddr string) {
	if connectAddr != "" {
		fullAddr, _ := multiaddr.NewMultiaddr(connectAddr)
		peerInfo, _ := peer.AddrInfoFromP2pAddr(fullAddr)
		err := h.Connect(context.Background(), *peerInfo)
		if err != nil {
			logger.Errorf("Failed to connect to peer %s: %v", connectAddr, err)
		} else {
			syncBlocksFromPeer(node, peerInfo.ID)
		}
		return
	}
	file, err := os.OpenFile(nodesAddrFile, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		logger.Error("Error opening json file: ", err)
		return
	}

	var n []NodeIdentifier
	decoder := json.NewDecoder(file)
	decoder.Decode(&n)
	if len(n) == 0 {
		return
	}

	rng := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	randomPeer := n[rng.Intn(len(n))]
	fullAddr, _ := multiaddr.NewMultiaddr(randomPeer.Addr)
	peerInfo, _ := peer.AddrInfoFromP2pAddr(fullAddr)
	err = h.Connect(context.Background(), *peerInfo)
	if err != nil {
		logger.Errorf("Error connecting to %s : %v\n", peerInfo.ID, err)
	} else {
		syncBlocksFromPeer(node, peerInfo.ID)
	}

	return
}
