package netstack

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	blkchn "github.com/shu8h0-null/minbit/core/blockchain"
)

const (
	topicBlock = "block"
	topicTx    = "transaction"
)

type NodePubSub struct {
	blockTopic *pubsub.Topic
	txTopic    *pubsub.Topic
	blockSub   *pubsub.Subscription
	txSub      *pubsub.Subscription
}

func NewNodePubSub(ctx context.Context, h host.Host) (*NodePubSub, error) {
	psOpts := []pubsub.Option{
		pubsub.WithMessageSigning(true),
	}

	ps, err := pubsub.NewGossipSub(ctx, h, psOpts...)
	if err != nil {
		log.Errorf("Error setting up instance for pubsub: %v", err)
		return nil, err
	}

	ps.RegisterTopicValidator(topicBlock, blockTopicValidator)
	ps.RegisterTopicValidator(topicTx, txTopicValidator)

	blockTopic, err := ps.Join(topicBlock)
	if err != nil {
		return nil, fmt.Errorf("Error joining to topic 'block': %v\n", err)
	}

	blockSub, err := blockTopic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("Error subscribing to topic 'block': %v\n", err)
	}

	txTopic, err := ps.Join(topicTx)
	if err != nil {
		return nil, fmt.Errorf("Error joingin to topic 'transaction': %v\n", err)
	}

	txSub, err := txTopic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("Error subscribing to topic 'transaction': %v\n", err)
	}

	nodePS := &NodePubSub{
		blockTopic: blockTopic,
		txTopic:    txTopic,
		blockSub:   blockSub,
		txSub:      txSub,
	}

	return nodePS, nil
}

func (nps *NodePubSub) BlockTopic() *pubsub.Topic {
	return nps.blockTopic
}

func (nps *NodePubSub) TxTopic() *pubsub.Topic {
	return nps.txTopic
}

func (nps *NodePubSub) BlockSub() *pubsub.Subscription {
	return nps.blockSub
}

func (nps *NodePubSub) TxSub() *pubsub.Subscription {
	return nps.txSub
}

func blockTopicValidator(ctx context.Context, pid peer.ID, blockMsg *pubsub.Message) bool {
	if len(blockMsg.Data) == 0 {
		log.Error("Invalid message: empty data")
		return false
	}

	var tx blkchn.Block
	dec := gob.NewDecoder(bytes.NewReader(blockMsg.Data))
	if err := dec.Decode(&tx); err != nil {
		log.Error("Invalid txMsg: Error decoding transaction message received from: %s\v", blockMsg.GetFrom(), err)
		return false
	}

	return true
}

func txTopicValidator(ctx context.Context, pid peer.ID, txMsg *pubsub.Message) bool {
	if len(txMsg.Data) == 0 {
		log.Error("Invalid message: empty data")
		return false
	}

	var tx blkchn.Transaction

	dec := gob.NewDecoder(bytes.NewReader(txMsg.Data))
	if err := dec.Decode(&tx); err != nil {
		log.Error("Invalid txMsg: Error decoding transaction message received from: %s\v", txMsg.GetFrom(), err)
		return false
	}
	if !tx.IsValid() {
		return false
	}

	return true
}
