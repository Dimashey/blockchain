package blockchain

import (
	"github.com/Dimashey/blockchain/internal/util"
	"github.com/dgraph-io/badger"
)

const dbPath = "./tmp/blocks"

type Chain struct {
	LastHash []byte
	Database *badger.DB
}

func (c *Chain) lastHash() ([]byte, error) {

	var lastHash []byte

	err := c.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))

		util.HandleError(err)

		lastHash, err = item.ValueCopy(nil)

		return err
	})

	if err != nil {
		return nil, err
	}

	return lastHash, nil
}

func (c *Chain) AddBlock(data string) {
	lastHash, err := c.lastHash()

	util.HandleError(err)

	newBlock := CreateBlock(data, lastHash)

	err = c.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())

		util.HandleError(err)

		err = txn.Set([]byte("lh"), newBlock.Hash)

		c.LastHash = newBlock.Hash

		return err
	})

	util.HandleError(err)
}

func InitBlockChain() *Chain {
	var lastHash []byte

	opts := badger.DefaultOptions(dbPath)

	db, err := badger.Open(opts)

	util.HandleError(err)

	err = db.Update(func(txn *badger.Txn) error {
		// Check if blockchain is exists
		if _, err := txn.Get([]byte("lh")); err == badger.ErrKeyNotFound {
			genesis := Genesis()

			err = txn.Set(genesis.Hash, genesis.Serialize())

			util.HandleError(err)

			err = txn.Set([]byte("lh"), genesis.Hash)

			lastHash = genesis.Hash

			return err
		}

		// Get last element hash in blockchain
		item, err := txn.Get([]byte("lh"))
		util.HandleError(err)

		lastHash, err = item.ValueCopy(nil)

		return err
	})

	return &Chain{lastHash, db}
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

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
