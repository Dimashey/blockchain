package blockchain

import (
	"github.com/Dimashey/blockchain/internal/util"
	"github.com/dgraph-io/badger"
)

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

// Iterator returns iterator which go through blockchain in reverse order from last to genesis block
func (c *Chain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{c.LastHash, c.Database}

	return iter
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		util.HandleError(err)

		encodedBlock, err := item.ValueCopy(nil)
		block = Deserialize(encodedBlock)

		return err
	})

	util.HandleError(err)

	iter.CurrentHash = block.PrevHash

	return block
}
