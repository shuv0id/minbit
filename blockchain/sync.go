package blockchain

import (
	"context"
	"encoding/gob"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type SyncRequest struct {
	LatestHeight int
}

type SyncResponse struct {
	Blocks []*Block
}

const syncProtocolID = "/blockchain/sync/1.0.0"

func handleSyncRequests(h host.Host) {
	h.SetStreamHandler(syncProtocolID, func(s network.Stream) {
		defer s.Close()
		var syncReq SyncRequest
		if err := gob.NewDecoder(s).Decode(&syncReq); err != nil {
			logger.Error("Error decoding sync request: ", err)
			return
		}

		// Send the requested blocks
		var resp SyncResponse
		if syncReq.LatestHeight == -1 {
			resp = SyncResponse{Blocks: bc.Chain}
		} else if syncReq.LatestHeight == bc.Chain[len(bc.Chain)-1].Height {
			resp = SyncResponse{}
		} else {
			resp = SyncResponse{
				Blocks: bc.Chain[syncReq.LatestHeight+1:],
			}
		}
		if err := gob.NewEncoder(s).Encode(resp); err != nil {
			logger.Error("Error sending sync response:", err)
		}

	})
}

func requestBlocks(h host.Host, peerID peer.ID, blockchainHeight int) ([]*Block, error) {
	s, err := h.NewStream(context.Background(), peerID, syncProtocolID)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	syncReq := SyncRequest{LatestHeight: blockchainHeight}

	if err := gob.NewEncoder(s).Encode(&syncReq); err != nil {
		return nil, err
	}

	var syncResp SyncResponse
	if err := gob.NewDecoder(s).Decode(&syncResp); err != nil {
		return nil, err
	}

	logger.Infof("Received %d blocks during sync", len(syncResp.Blocks))
	return syncResp.Blocks, nil
}

func syncBlocksFromPeer(h host.Host, peerID peer.ID) {
	blockchainHeight := bc.GetLatestBlockHeight()
	blocks, err := requestBlocks(h, peerID, blockchainHeight)
	if err != nil {
		logger.Error("Error syncing blocks: ", err)
	}

	if len(blocks) == 0 {
		return
	}
	for _, block := range blocks {
		if block.isValid() {
			if err := bc.AddBlock(block); err != nil {
				logger.Errorf("Error adding block: %v\n", err)
			} else {
				logger.Successf("Block:[%d]:[%s] added", block.Height, block.Hash)
			}
			if err = utxoSet.Update(); err != nil {
				logger.Errorf("Error adding updating utxoSet: %v\n", err)
			} else {
				logger.Success("UTXOSet updated")
			}

			for _, tx := range block.TxData {
				if !tx.IsCoinbase {
					mempool.RemoveTransaction(tx.TxID)
				}
			}

		}
	}
}
