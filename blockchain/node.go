package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var node host.Host

const (
	NodesAddrFile    = "nodes.json" // this file will be used by nodes on bootup for peer discovery if no peer address
	topicBlock       = "block"
	topicTransaction = "transaction"
)

type NodeIdentifier struct {
	PeerID string `json:"peer_id"`
	Addr   string `json:"address"`
}

func StartNode(ctx context.Context, port int, randseed int64, connectAddr string) error {
	if port == 0 {
		const maxPort = 65535
		for port = 6969; port <= maxPort; port++ {
			if CheckPortAvailability("localhost", port) {
				break
			}
		}
	}

	priv, err := GeneratePrivKeyForNode(randseed)
	if err != nil {
		logger.Error(err)
		return err
	}

	NodeWallet = GenerateWallet()

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)),
		libp2p.Identity(priv),
	}

	node, err = libp2p.New(opts...)
	if err != nil {
		logger.Error(err)
		return err
	}

	// subscribing
	nodePS, err := applyPubSub(ctx, node)
	if err != nil {
		logger.Errorf("Error setting up instance for pubsub: %v", err)
		return err
	}

	nodePS.RegisterTopicValidator(topicBlock, bTopicValidator)
	nodePS.RegisterTopicValidator(topicTransaction, txTopicValidator)

	bTopic, err := nodePS.Join(topicBlock)
	if err != nil {
		logger.Errorf("Error subscribing to topic 'block': %v", err)
		return err
	}
	bSub, _ := bTopic.Subscribe()

	txTopic, err := nodePS.Join(topicTransaction)
	if err != nil {
		logger.Errorf("Error subscribing to topic 'transaction': %s\n", err)
		return err
	}
	txSub, _ := txTopic.Subscribe()

	blockReceiver := make(chan *Block, 1)
	go blockReader(ctx, bSub, blockReceiver)
	go txReader(ctx, txSub)

	node.Network().Notify(&MyNotifiee{})

	fullAddr := node.Addrs()[0].String() + "/p2p/" + node.ID().String()
	logger.Successf("Node started at address: %s\n", fullAddr)
	connectToPeer(ctx, node, connectAddr)
	writeNodeAddrToJSONFile(fullAddr, node.ID().String(), NodesAddrFile)

	// sync blocks from connected peers on node startup
	handleSyncRequests(node)

	go MineBlocks(ctx, blockReceiver, bTopic)
	go HandleNodeCommands(ctx, txTopic)

	// Signal handling for proper shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM)

	select {
	case quitSig := <-quit:
		logger.Info("Received signal: ", quitSig)
		if err := removeNodeInfoFromJSONFile(node.ID().String(), NodesAddrFile); err != nil {
			logger.Error("Error removing the peer address from json file:", err)
		}
		logger.Error("Node shutting down...")
		node.Close()
		return nil
	}
}

func applyPubSub(ctx context.Context, h host.Host) (*pubsub.PubSub, error) {
	psOpts := []pubsub.Option{
		pubsub.WithMessageSigning(true),
	}

	return pubsub.NewGossipSub(ctx, h, psOpts...)
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

func blockReader(ctx context.Context, bSub *pubsub.Subscription, blockReceiver chan<- *Block) {
	for {
		bMsg, err := bSub.Next(ctx)
		if err != nil {
			logger.Error("Error receiving next message: ")
		}

		if bMsg.GetFrom() == node.ID() {
			continue
		}
		block := &Block{}

		json.Unmarshal(bMsg.Data, &block)
		logger.Infof("Received block with id: %s from: %s\n", block.Hash, bMsg.GetFrom())

		if !block.isValid() {
			continue
		}

		// Signals miner with receiving block
		blockReceiver <- block

		us.update(block.TxData)

		for _, tx := range block.TxData {
			if !tx.IsCoinbase {
				mempool.RemoveTransaction(tx.TxID)
			}
		}

		bc.AddBlock(block)
	}
}

func txReader(ctx context.Context, txSub *pubsub.Subscription) {
	for {
		txMsg, _ := txSub.Next(ctx)
		if txMsg.GetFrom() == node.ID() {
			continue
		}
		tx := &Transaction{}

		json.Unmarshal(txMsg.Data, &tx)
		logger.Infof("Received transaction with id: %s from %s\n", tx.TxID, txMsg.GetFrom())
		mempool.AddTransaction(tx)
	}
}

// connectToPeer establishes a connection between the host and a peer node with the provided connectAddr
func connectToPeer(ctx context.Context, h host.Host, connectAddr string) {
	if connectAddr != "" {
		fullAddr, _ := multiaddr.NewMultiaddr(connectAddr)
		peerInfo, _ := peer.AddrInfoFromP2pAddr(fullAddr)
		err := h.Connect(ctx, *peerInfo)
		if err != nil {
			logger.Errorf("Failed to connect to peer %s: %v", connectAddr, err)
		} else {
			syncBlocksFromPeer(node, peerInfo.ID)
		}
	} else {
		peerInfo := GetRandomPeerInfo()
		if peerInfo != nil {
			err := h.Connect(context.Background(), *peerInfo)
			if err != nil {
				logger.Errorf("Failed to connect to peer %s: %v", connectAddr, err)
			} else {
				syncBlocksFromPeer(node, peerInfo.ID)
			}
		}
	}
}

// GetRandomPeerInfo is a helper function for getting peerInfo from a random peer from NodesAddrFile
func GetRandomPeerInfo() *peer.AddrInfo {
	file, err := os.OpenFile(NodesAddrFile, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		logger.Error("Error opening json file: ", err)
		return nil
	}

	var n []NodeIdentifier
	decoder := json.NewDecoder(file)
	decoder.Decode(&n)
	if len(n) == 0 {
		return nil
	}

	rng := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	randomPeer := n[rng.Intn(len(n))]
	fullAddr, _ := multiaddr.NewMultiaddr(randomPeer.Addr)
	peerInfo, _ := peer.AddrInfoFromP2pAddr(fullAddr)
	return peerInfo
}
