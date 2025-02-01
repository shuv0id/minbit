package blockchain

import (
	"context"
	"encoding/hex"
	"encoding/json"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

type WalletReq struct {
	Address string `json:"address"`
}

type WalletResp struct {
	Resp     any
	ErrorMsg string
}

const UtxoSetReqProtocolID = "/blockchain/utxoreq/1.0.0"
const TxReqProtocolID = "/blockchain/txhandler/1.0.0"

// utxoRequestHandler sets a stream handler on the node for getting available UTXOsafor given address
func utxoRequestHandler(h host.Host) {
	h.SetStreamHandler(UtxoSetReqProtocolID, func(s network.Stream) {
		defer s.Close()

		var walletReq WalletReq
		var walletResp WalletResp
		var respUTXOSet UTXOSet
		respUTXOSet.UTXOs = make(map[string]map[int]UTXO)

		err := json.NewDecoder(s).Decode(&walletReq)
		if err != nil {
			logger.Error("Error decoding wallet request: ", err)
			walletResp = WalletResp{ErrorMsg: "Bad Request"}
			json.NewEncoder(s).Encode(&walletResp)
			return
		}

		for _, transactions := range utxoSet.UTXOs {
			for _, utxo := range transactions {
				pubKeyHash, err := hex.DecodeString(utxo.ScriptPubKey)
				if err != nil {
					logger.Warn("Error decoding scriptPubKey: ", err)
					continue
				}

				utxoAddr := PubKeyHashToAddress(pubKeyHash)

				if utxoAddr == walletReq.Address {
					if _, exists := respUTXOSet.UTXOs[utxo.TxID]; !exists {
						respUTXOSet.UTXOs[utxo.TxID] = make(map[int]UTXO)
					}
					respUTXOSet.UTXOs[utxo.TxID][utxo.OutputIndex] = utxo
				}
			}
		}

		walletResp = WalletResp{Resp: respUTXOSet}
		err = json.NewEncoder(s).Encode(walletResp)
		if err != nil {
			logger.Error("Error sending inputs for transaction:", err)
			return
		}
	})
}

// txReqHandler sets a stream handler on the node for handling new incoming transaction from a light wallet
func txReqHandler(h host.Host, txPublisher *pubsub.Topic) {
	h.SetStreamHandler(TxReqProtocolID, func(s network.Stream) {
		defer s.Close()
		var newTx Transaction
		var walletResp WalletResp

		err := json.NewDecoder(s).Decode(&newTx)
		if err != nil {
			logger.Error("Error decoding incoming transaction: ", err)
			walletResp = WalletResp{ErrorMsg: "Bad Request!"}
			json.NewEncoder(s).Encode(walletResp)
			return
		}

		newTxBytes, err := json.Marshal(newTx)
		if err != nil {
			logger.Error("Error marshaling transaction: ", err)
			walletResp = WalletResp{ErrorMsg: "Internal server error"}
			json.NewEncoder(s).Encode(walletResp)
			return
		}

		err = txPublisher.Publish(context.Background(), newTxBytes)
		if err != nil {
			logger.Error("Error publishing new incoming transaction: ", err)
			walletResp = WalletResp{ErrorMsg: "Internal server error"}
			json.NewEncoder(s).Encode(walletResp)
			return
		}

		walletResp = WalletResp{Resp: "OK"}
		json.NewEncoder(s).Encode(walletResp)
	})
}
