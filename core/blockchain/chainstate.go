package blockchain

import "errors"

type ChainState struct {
	blockchain *Blockchain
	utxoSet    *UTXOSet
	mempool    *Mempool
}

func NewChainState(bc *Blockchain, us *UTXOSet, mem *Mempool) (*ChainState, error) {
	if bc == nil {
		return nil, errors.New("Blockchain cannot be nil")
	}
	if us == nil {
		return nil, errors.New("UTXOSet cannnot be nil")
	}
	if mem == nil {
		return nil, errors.New("Mempool cannot be nil")
	}

	return &ChainState{
		blockchain: bc,
		utxoSet:    us,
		mempool:    mem,
	}, nil
}

func (cs *ChainState) Blockchain() *Blockchain {
	return cs.blockchain
}
func (cs *ChainState) UTXOSet() *UTXOSet {
	return cs.utxoSet
}
func (cs *ChainState) Mempool() *Mempool {
	return cs.mempool
}
