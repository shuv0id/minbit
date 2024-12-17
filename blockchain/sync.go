package blockchain

import (
	"context"
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

type SyncRequest struct {
	FromHeight int `json:"from_height"`
}

type SyncResponse struct {
	Blocks []*Block `json:"blocks"`
}

const syncProtocolID = "/blockchain/sync/1.0.0"

func handleSyncRequests(h host.Host) {
	h.SetStreamHandler(syncProtocolID, func(s network.Stream) {
		defer s.Close()
		var syncReq SyncRequest
		if err := json.NewDecoder(s).Decode(&syncReq); err != nil {
			logger.Error("Error decoding sync request: ", err)
			return
		}

		// Send the requested blocks
		var resp SyncResponse
		if syncReq.FromHeight == -1 {
			resp = SyncResponse{Blocks: bc.Chain}
		} else {
			resp = SyncResponse{
				Blocks: bc.Chain[syncReq.FromHeight:],
			}
		}
		if err := json.NewEncoder(s).Encode(resp); err != nil {
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

	syncReq := SyncRequest{FromHeight: blockchainHeight}

	if err := json.NewEncoder(s).Encode(&syncReq); err != nil {
		return nil, err
	}

	var syncResp SyncResponse
	if err := json.NewDecoder(s).Decode(&syncResp); err != nil {
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

	for _, block := range blocks {
		if block.isValid() {
			bc.AddBlock(block)
		}
	}
}
