package rpc

import "github.com/shu8h0-null/minbit/core/blockchain"

type server interface {
	GetBlockByHash(hash string) *blockchain.Block
	GetBlockByHeight(height uint64) *blockchain.Block
}
