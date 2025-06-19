package core

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	blkchn "github.com/shu8h0-null/minbit/core/blockchain"
	"github.com/shu8h0-null/minbit/core/logger"
	"github.com/shu8h0-null/minbit/core/netstack"
)

var (
	EventBus         = blkchn.NewEventBus()
	ErrNoOnlinePeers = errors.New("No online peers found")
)

type Node struct {
	host       host.Host
	pubSub     *netstack.NodePubSub
	store      *blkchn.Store
	chainState *blkchn.ChainState
	miner      *blkchn.Miner
}

type SyncRequest struct {
	BlkchnHeight int
}

type SyncResponse struct {
	Blocks []*blkchn.Block
}

const (
	blockBucket = "Blocks"
	utxoBucket  = "Utxos"
)

const syncProtocolID = "/blockchain/sync/1.0.0"

var log = logger.NewLogger()

func NewNode(h host.Host, nps *netstack.NodePubSub, store *blkchn.Store, cs *blkchn.ChainState, miner *blkchn.Miner) (*Node, error) {
	if h == nil {
		return nil, errors.New("Host cannot be nil")
	}
	if nps == nil {
		return nil, errors.New("Pubsub cannot be nil")
	}
	if store == nil {
		return nil, errors.New("Store cannot be nil")
	}
	if cs == nil {
		return nil, errors.New("Chainstate cannot be nil")
	}

	node := &Node{
		host:       h,
		pubSub:     nps,
		store:      store,
		chainState: cs,
		miner:      miner,
	}
	return node, nil
}

func (n *Node) PublishBlock(ctx context.Context, block *blkchn.Block) error {
	var blockBytes bytes.Buffer
	err := gob.NewEncoder(&blockBytes).Encode(block)
	if err != nil {
		return fmt.Errorf("Error marshalling block to gob %v\n", err)
	}

	err = n.pubSub.BlockTopic().Publish(ctx, blockBytes.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (n *Node) PublishTx(ctx context.Context, tx *blkchn.Transaction) error {
	var txBytes bytes.Buffer
	err := gob.NewEncoder(&txBytes).Encode(tx)
	if err != nil {
		return fmt.Errorf("Error marshalling block to gob %v\n", err)
	}

	err = n.pubSub.TxTopic().Publish(ctx, txBytes.Bytes())
	if err != nil {
		return err
	}

	return nil
}

// RunMiner is an indefinetly running function that contantly mine new blocks
func (n *Node) RunMiner(ctx context.Context) {
	bc := n.chainState.Blockchain()
	mem := n.chainState.Mempool()
	for {
		var txs []blkchn.Transaction
		coinbaseTx := n.miner.GenerateCoinbaseTx()
		memTxs := n.miner.CollectTransactions(mem, 3) // NOTE: max transaction to collect hardcoded for now.. idk what to do with it
		txs = append(txs, coinbaseTx)
		txs = append(txs, memTxs...)
		block := n.chainState.Blockchain().NewBlock(txs)

		log.Infof("Mining for new Block:[%d]\n", block.Height)
		minedBlock := n.miner.MineBlock(block, bc.Difficulty())

		if minedBlock != nil {
			log.Infof("Hell yeah!! Block:[%d]:[%s] mined\n", minedBlock.Height, minedBlock.Hash)
			if err := n.PublishBlock(ctx, minedBlock); err != nil {
				log.Info("Block:[%d]:[%s] published over the network\n", minedBlock.Height, minedBlock.Hash)
			}
			if err := n.FinalizeBlock(minedBlock); err != nil {
				log.Errorf("Failed to finalize block: %v\n", err)
			} else {
				log.Infof("Block:[%d]:[%s] finalized\n", minedBlock.Height, minedBlock.Hash)
			}

		} else {
			fmt.Println("We are here")
			log.Infof("Mining aborted for the current block, Block with same height[%d] received", block.Height)
		}

	}
}

func (n *Node) Close() error {
	if err := n.host.Close(); err != nil {
		return fmt.Errorf("Error closing host: %v\n", err)
	}
	if err := n.store.Close(); err != nil {
		return fmt.Errorf("Error closing db: %v\n", err)
	}
	return nil
}

func (n *Node) BlockReader(ctx context.Context) {
	for {
		blockMsg, err := n.pubSub.BlockSub().Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("Error receiving next block message: ")
		}

		if blockMsg.GetFrom() == n.host.ID() {
			continue
		}

		var block blkchn.Block

		dec := gob.NewDecoder(bytes.NewReader(blockMsg.Data))
		if err := dec.Decode(&block); err != nil {
			log.Error("Error decoding block message received from: %s\v", blockMsg.GetFrom(), err)
			continue
		}

		log.Infof("Received block:[%d]:[%s]: from %s\n", block.Height, block.Hash, blockMsg.GetFrom())

		if err := n.FinalizeBlock(&block); err != nil {
			log.Errorf("Failed to finalize block: %v\n", err)
		} else {
			log.Infof("Block:[%d]:[%s] finalized\n", block.Height, block.Hash)
			blkRecEvent := blkchn.BlockRecEvent{BlkHeight: block.Height}
			EventBus.BlockFeed.Send(blkRecEvent)
		}
	}
}

func (n *Node) TxReader(ctx context.Context) {
	for {
		txMsg, err := n.pubSub.TxSub().Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("Error receiving next transaction message: ")
		}

		if txMsg.GetFrom() == n.host.ID() {
			continue
		}
		var tx blkchn.Transaction

		dec := gob.NewDecoder(bytes.NewReader(txMsg.Data))
		if err := dec.Decode(&tx); err != nil {
			log.Error("Error decoding transaction message received from: %s\v", txMsg.GetFrom(), err)
			continue
		}
		log.Infof("Received transaction with id: %s from %s\n", tx.TxID, txMsg.GetFrom())
		n.chainState.Mempool().AddTx(&tx)

	}
}

// GetRandomPeerAddr is a helper function for getting peerInfo from a random peer
func (n *Node) GetRandomPeerAddr() (string, error) {
	peers, err := netstack.ReadOnlinePeers()
	if err != nil && err != io.EOF {
		return "", err
	}

	delete(peers, n.host.ID().String())

	if len(peers) == 0 {
		return "", ErrNoOnlinePeers
	}

	onlinePeersAddr := make([]string, 0, len(peers))
	for _, addr := range peers {
		onlinePeersAddr = append(onlinePeersAddr, addr)
	}

	seed := int64(time.Now().Nanosecond())
	rng := rand.New(rand.NewSource(seed))
	randomPeerAddr := onlinePeersAddr[rng.Intn(len(onlinePeersAddr))]

	return randomPeerAddr, nil
}

func (n *Node) FinalizeBlock(block *blkchn.Block) error {
	bc := n.chainState.Blockchain()
	us := n.chainState.UTXOSet()
	mem := n.chainState.Mempool()

	if !bc.IsValid(block) {
		return errors.New("Skipping to add block: Invalid block generated by miner")
	}

	if err := bc.AddBlock(block); err != nil {
		return err
	}

	if err := us.Update(block.TxData); err != nil {
		return err
	}
	for _, tx := range block.TxData {
		if tx.IsCoinbase == false {
			mem.RemoveTx(tx.TxID)
		}
	}

	return nil
}

func (n *Node) HandleSyncRequests() {
	n.host.SetStreamHandler(syncProtocolID, func(s network.Stream) {
		defer s.Close()
		var syncReq SyncRequest
		if err := gob.NewDecoder(s).Decode(&syncReq); err != nil {
			log.Error("Error decoding sync request: ", err)
			return
		}

		// Send the requested blocks
		bc := n.chainState.Blockchain()
		blkchnHeight := n.chainState.Blockchain().GetBlockchainHeight()
		var resp SyncResponse
		if syncReq.BlkchnHeight == -1 {
			resp = SyncResponse{Blocks: bc.Chain()}
		} else if syncReq.BlkchnHeight == blkchnHeight {
			resp = SyncResponse{}
		} else {
			resp = SyncResponse{
				Blocks: bc.Chain()[syncReq.BlkchnHeight+1:],
			}
		}
		if err := gob.NewEncoder(s).Encode(resp); err != nil {
			log.Error("Error sending sync response:", err)
		}

	})
}

func (node *Node) requestBlocks(ctx context.Context, peerID peer.ID, blkchnHeight int) ([]*blkchn.Block, error) {
	s, err := node.host.NewStream(ctx, peerID, syncProtocolID)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	syncReq := SyncRequest{BlkchnHeight: blkchnHeight}

	if err := gob.NewEncoder(s).Encode(&syncReq); err != nil {
		return nil, err
	}

	var syncResp SyncResponse
	if err := gob.NewDecoder(s).Decode(&syncResp); err != nil {
		return nil, err
	}

	log.Infof("Received %d blocks during sync", len(syncResp.Blocks))
	return syncResp.Blocks, nil
}

func (n *Node) SyncBlocksFromPeer(ctx context.Context, peerID peer.ID, blockchainHeight int) error {
	blocks, err := n.requestBlocks(ctx, peerID, blockchainHeight)
	if err != nil {
		return err
	}

	if len(blocks) == 0 {
		return nil
	}

	for _, block := range blocks {
		if err := n.FinalizeBlock(block); err != nil {
			log.Error("Error finalising received block:[%d]:[%s] during sync: %v\n", block.Height, block.Hash, err)
		} else {
			log.Infof("Block:[%d]:[%s] finalized\n", block.Height, block.Hash)
		}
	}
	return nil
}

func InitNode(ctx context.Context, port int, id string, randseed int64, mine bool) (*Node, error) {
	var err error
	if id != "" {
		onlinePeers, err := netstack.ReadOnlinePeers()
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("Error reading peersInfo file: %v\n", err)
		}
		if _, exists := onlinePeers[id]; exists {
			return nil, fmt.Errorf("Node with given id is already running.\n")
		}
	}

	var priv crypto.PrivKey
	if id != "" {
		priv, err = netstack.LoadNodePrivKey(id)
		if err != nil {
			return nil, fmt.Errorf("No node found for the given id: ", err)
		}
	} else {
		priv, err = netstack.GeneratePrivKeyForNode(randseed)
		if err != nil {
			return nil, fmt.Errorf("Error generating private key for node: ", err)
		}
	}

	h, err := netstack.NewHost(ctx, port, priv)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialise host %v\n", err)
	}
	err = netstack.SaveHostAddrToFile(h)
	if err != nil {
		log.Error(err)
	}

	nps, err := netstack.NewNodePubSub(ctx, h)
	if err != nil {
		return nil, fmt.Errorf("Error initialising pubsub %v\n", err)
	}

	store, err := initStore(h.ID())
	if err != nil {
		return nil, fmt.Errorf("Could not initialise db for blockchain: %v\n", err)
	}

	cs, err := initChainState(store)
	if err != nil {
		return nil, fmt.Errorf("Error creating new chainstate: %v\n", err)
	}

	var miner *blkchn.Miner
	if mine {
		var wallet *blkchn.Wallet
		wallet, err = blkchn.LoadWallet(h.ID().String())
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				wallet, err = blkchn.NewWallet(h.ID().String())
				if err != nil {
					return nil, fmt.Errorf("Error creating wallet for miner\n%v", err)
				}
			} else {
				return nil, fmt.Errorf("Error loading wallet for miner\n%v", err)
			}
		}

		be := make(chan blkchn.BlockRecEvent, 1)
		EventBus.BlockFeed.Subscribe("miner", be)
		miner, err = initMiner(be, wallet)
		if err != nil {
			return nil, err
		}
	}

	n, err := NewNode(h, nps, store, cs, miner)
	if err != nil {
		return nil, err
	}
	err = netstack.SaveHostPrivKey(n.host.ID(), priv)
	if err != nil {
		log.Errorf("Error saving priv key of node %v\n", err)
	}
	return n, err
}

func (n *Node) Connect(ctx context.Context, target string) error {
	addr := target

	if addr == "" {
		var err error
		addr, err = n.GetRandomPeerAddr()
		if err != nil {
			return err
		}
	}

	if addr != "" {
		if err := netstack.ConnectToPeer(ctx, n.host, addr); err != nil {
			return fmt.Errorf("Error connecting to peer [%s]: %v\n", addr, err)
		} else {
			peerID, err := netstack.ExtractPeerID(addr)
			if err != nil {
				return fmt.Errorf("Error extracting peer id from the peer address:[%s]: %v\n", addr, err)
			}
			blockchainHeight := n.chainState.Blockchain().GetBlockchainHeight()
			err = n.SyncBlocksFromPeer(ctx, peerID, blockchainHeight)
			if err != nil {
				return fmt.Errorf("Error syncing blocks: %v\n", err)
			}
		}
	}
	return nil
}

func (n *Node) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	listenForQuitSignal(ctx, cancel)

	go n.TxReader(ctx)
	go n.BlockReader(ctx)
	if n.miner != nil {
		go n.RunMiner(ctx)
	}
	n.HandleSyncRequests()

	var addr ma.Multiaddr
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", n.host.ID().String()))
	addrs := n.host.Addrs()
	for _, a := range addrs {
		if strings.HasPrefix(a.String(), "/ip4") {
			addr = a
			break
		}
	}
	fullAddr := addr.Encapsulate(hostAddr)
	log.Infof("Node::%s started at %s\n", n.host.ID().String(), fullAddr)

	<-ctx.Done()
	log.Info("Cleaning Up...")
	if err := netstack.RemoveHostMultiaddrFromFile(n.host.ID()); err != nil {
		return fmt.Errorf("Error removing host address from peers file: %v\n", err)
	}
	if err := n.Close(); err != nil {
		return fmt.Errorf("Error closing node: %v\n", err)
	}

	return nil
}

func initStore(hostID peer.ID) (*blkchn.Store, error) {
	store, err := blkchn.NewDb(hostID.String())
	if err != nil {
		return nil, err
	}

	err = store.CreateBlocksBucket()
	if err != nil {
		return nil, err
	}

	err = store.CreateUTXOSetBucket()
	if err != nil {
		return nil, err
	}

	err = store.CreateTxIndexBucket()
	if err != nil {
		return nil, err
	}

	return store, nil
}

func initChainState(store *blkchn.Store) (*blkchn.ChainState, error) {
	bc, err := blkchn.NewBlockchain(store, blockBucket)
	if err != nil {
		return nil, fmt.Errorf("Could not create blockchain: %v\n", err)
	}

	us, err := blkchn.NewUTXOSet(store, utxoBucket)
	if err != nil {
		return nil, fmt.Errorf("Could not create utxoset: %v\n", err)
	}

	mem := blkchn.NewMempool()

	cs, err := blkchn.NewChainState(bc, us, mem)
	if err != nil {
		return nil, fmt.Errorf("Error creating new chainstate: %v\n", err)
	}

	return cs, nil
}

func initMiner(bre <-chan blkchn.BlockRecEvent, minerWallet *blkchn.Wallet) (*blkchn.Miner, error) {
	miner, err := blkchn.NewMiner(minerWallet, bre)
	if err != nil {
		return nil, fmt.Errorf("Error initialising miner %v\n", err)
	}
	return miner, nil
}

func listenForQuitSignal(ctx context.Context, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigCh:
			log.Infof("Received signal: %s, shutting down...", sig)
			cancel() // Trigger context cancellation
		}
	}()
}

func (n *Node) GetBlockByHash(hash string) *blkchn.Block {
	var block blkchn.Block

	index := n.chainState.Blockchain().Index()

	if blockHeight, exists := index[hash]; !exists {
		return nil
	} else {
		chain := n.chainState.Blockchain().Chain()
		block = *chain[blockHeight]
	}

	return &block
}

func (n *Node) GetBlockByHeight(height uint64) *blkchn.Block {
	var block blkchn.Block
	chain := n.chainState.Blockchain().Chain()

	if len(chain) == 0 || height+1 > uint64(len(chain)) {
		return nil
	}

	block = *chain[int(height)]
	if block.Height != height {
		return nil
	}

	return &block
}
