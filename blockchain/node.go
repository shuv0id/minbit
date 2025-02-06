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
	PeersInfoFile = "peers.json" // this file will be used by nodes on bootup for peer discovery
	topicBlock    = "block"
	topicTx       = "transaction"
)

type PeersInfo map[string]string // peers maps a peer ID (string) to its full P2P address (string)

func StartNode(ctx context.Context, port int, randseed int64, connectAddr string) error {
	if port == 0 {
		logger.Error("Please provide a valid port!")
		return fmt.Errorf("Please provide a valid port!")
	} else if !CheckPortAvailability("localhost", port) {
		logger.Errorf("Port: %d is busy!", port)
		return fmt.Errorf("Port: %d is busy!", port)
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
	nodePS.RegisterTopicValidator(topicTx, txTopicValidator)

	bTopic, err := nodePS.Join(topicBlock)
	if err != nil {
		logger.Errorf("Error subscribing to topic 'block': %v", err)
		return err
	}
	bSub, _ := bTopic.Subscribe()

	txTopic, err := nodePS.Join(topicTx)
	if err != nil {
		logger.Errorf("Error subscribing to topic 'transaction': %s\n", err)
		return err
	}
	txSub, _ := txTopic.Subscribe()

	blockReceiver := make(chan *Block)
	go blockReader(ctx, bSub, blockReceiver)
	go txReader(ctx, txSub)

	node.Network().Notify(conn_notifiee)

	fullAddr := node.Addrs()[0].String() + "/p2p/" + node.ID().String()
	logger.Successf("Node started at address: %s\n", fullAddr)

	connectToPeer(ctx, node, connectAddr)
	writePeerInfoToJSONFile(fullAddr, node.ID().String(), PeersInfoFile)

	// sync blocks from connected peers on node startup
	handleSyncRequests(node)

	// wallet req handlers
	utxoRequestHandler(node)
	txReqHandler(node, txTopic)

	go MineBlocks(ctx, blockReceiver, bTopic)
	go HandleNodeCommands(ctx, txTopic)

	// Signal handling for proper shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM)

	select {
	case quitSig := <-quit:
		logger.Info("Received signal: ", quitSig)
		if err := removePeerInfoFromJSONFile(node.ID().String(), PeersInfoFile); err != nil {
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

		utxoSet.update(block.TxData)

		for _, tx := range block.TxData {
			if !tx.IsCoinbase {
				mempool.RemoveTransaction(tx.TxID)
			}
		}

		bc.AddBlock(block)

		// Signals miner with receiving block
		blockReceiver <- block
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

// connectToPeer establishes a connection between the host and a peer node with the provided connectAddr(if provided)
// else connects to a random peer from `PeersInfoFile`
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

// GetRandomPeerInfo is a helper function for getting peerInfo from a random peer from NodeList
func GetRandomPeerInfo() *peer.AddrInfo {
	peers, err := ReadPeersInfo()
	if err != nil {
		logger.Error(err)
		return nil
	}

	fullAddrs := make([]string, 0, len(peers))
	for _, addr := range peers {
		fullAddrs = append(fullAddrs, addr)
	}
	rng := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	randomPeerAddr := fullAddrs[rng.Intn(len(fullAddrs))]
	fullAddr, _ := multiaddr.NewMultiaddr(randomPeerAddr)
	peerInfo, _ := peer.AddrInfoFromP2pAddr(fullAddr)
	return peerInfo
}

// ReadPeersInfo reads nodes from `PeersInfoFile` (creating the file if it doesn't exist), returns PeersInfo or an error.
func ReadPeersInfo() (PeersInfo, error) {
	var peers PeersInfo
	file, err := os.OpenFile(PeersInfoFile, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("Failed to open file %s: %v\n", PeersInfoFile, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&peers)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode json: %v", err)
	}

	if len(peers) == 0 {
		return nil, fmt.Errorf("No peerInfo found in the file")
	}

	return peers, nil
}
