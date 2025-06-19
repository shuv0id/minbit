package rpc

import (
	"net/http"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/shu8h0-null/minbit/core/blockchain"
)

type RPCHandler struct {
	rpcServer server
}

func NewRPCHandler(s server) *RPCHandler {
	return &RPCHandler{
		rpcServer: s,
	}
}

func (h RPCHandler) GetBlockByHash(hash string) *blockchain.Block {
	return h.rpcServer.GetBlockByHash(hash)
}

func (h RPCHandler) GetBlockByHeight(height uint64) *blockchain.Block {
	return h.rpcServer.GetBlockByHeight(height)
}

func StartRPC(addr string, handler *RPCHandler) error {
	mux := http.NewServeMux()
	rpcServer := jsonrpc.NewServer()
	rpcServer.Register("NodeRPC", handler)
	mux.Handle("/rpc/v0", rpcServer)
	return http.ListenAndServe(addr, mux)
}
