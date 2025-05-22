package core

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	blkchn "github.com/shu8h0-null/minbit/core/blockchain"
	"github.com/shu8h0-null/minbit/core/logger"
	"github.com/shu8h0-null/minbit/core/netstack"
	"golang.org/x/net/context"
)

type Node struct {
	host       host.Host
	pubSub     *netstack.NodePubSub
	chainState *blkchn.ChainState
	logger     *logger.Logger
	miner      *blkchn.Miner
}

type SyncRequest struct {
	BlkchnHeight int
}

type SyncResponse struct {
	Blocks []*blkchn.Block
}

const syncProtocolID = "/blockchain/sync/1.0.0"

var log = logger.NewLogger()

func NewNode(ctx context.Context, port int, priv crypto.PrivKey) (*Node, error) {
	h, err := netstack.NewHost(ctx, port, priv)
	if err != nil {
		return nil, err
	}

	nps, err := netstack.NewNodePubSub(ctx, h)
	if err != nil {
		return nil, err
	}
	node := &Node{
		host:   h,
		pubSub: nps,
	}
	return node, nil
}

func (n *Node) SetChainState(cs *blkchn.ChainState) error {
	if cs == nil {
		return errors.New("Nil ChainState provided")
	}
	n.chainState = cs
	return nil
}

func (n *Node) SetMiner(miner *blkchn.Miner) error {
	if miner == nil {
		return errors.New("Nil miner provided")
	}
	n.miner = miner
	return nil
}

func (n *Node) ID() peer.ID {
	return n.host.ID()
}

func (n *Node) Host() host.Host {
	return n.host
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

// Mine is an indefinetly running function that contantly mine new blocks
func (n *Node) Mine(ctx context.Context) {
	bc := n.chainState.Blockchain()
	mem := n.chainState.Mempool()
	for {
		txs := n.miner.CollectTransactions(mem, 3) // NOTE: max transaction to collect hardcoded for now.. idk what to do with it
		block := n.chainState.Blockchain().NewBlock(txs)

		log.Infof("Mining for new Block:[%d]\n", block.Height)
		block, mined := n.miner.MineBlock(block, bc.Difficulty())

		if mined {
			log.Infof("Hell yeah!! Block:[%d]:[%s] mined\n", block.Height, block.Hash)
			if err := n.PublishBlock(ctx, block); err != nil {
				log.Info("Block:[%d]:[%s] published over the network\n", block.Height, block.Hash)
			}
			if err := n.FinalizeBlock(block); err != nil {
				log.Errorf("Failed to finalize block: %v\n", err)
			} else {
				log.Infof("Block:[%d]:[%s] finalized\n", block.Height, block.Hash)
			}

		} else {
			log.Infof("Mining aborted for the current block, Block with same height[%d] received", block.Height)
		}

	}
}

func (n *Node) Close() error {
	if err := n.host.Close(); err != nil {
		return fmt.Errorf("Error closing node: %v\n", err)
	}
	return nil
}

func (n *Node) BlockReader(ctx context.Context, blockCh chan *blkchn.Block) {
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
		}

		blockCh <- &block // signal miner for incoming block
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
		log.Error(err)
		return "", err
	}

	delete(peers, n.ID().String())

	if len(peers) == 0 {
		return "", errors.New("No online peers found")
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
		mem.RemoveTx(tx.TxID)
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
