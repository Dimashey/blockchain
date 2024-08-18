package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/Dimashey/blockchain/internal/util"
	"github.com/dgraph-io/badger"
)

const dbPath = "./tmp/blocks"

// Manifest file which badger creates when intialize database
const dbFile = "./tmp/blocks/MANIFEST"
const genesisData = "First Transaction from Genesis"

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

func (c *Chain) AddBlock(txs []*Transaction) *Block {
	lastHash, err := c.lastHash()

	util.HandleError(err)

	newBlock := CreateBlock(txs, lastHash)

	err = c.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())

		util.HandleError(err)

		err = txn.Set([]byte("lh"), newBlock.Hash)

		c.LastHash = newBlock.Hash

		return err
	})

	util.HandleError(err)

	return newBlock
}

func DBexists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}

func InitBlockChain(address string) *Chain {
	var lastHash []byte

	if DBexists() {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions(dbPath)

	db, err := badger.Open(opts)

	util.HandleError(err)

	err = db.Update(func(txn *badger.Txn) error {
		// Check if blockchain is exists
		if _, err := txn.Get([]byte("lh")); err == badger.ErrKeyNotFound {
			cbtx := CoinbaseTx(address, genesisData)
			genesis := Genesis(cbtx)

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

func ContinueBlockChain(address string) *Chain {
	if DBexists() == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions(dbFile)

	db, err := badger.Open(opts)

	util.HandleError(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))

		util.HandleError(err)

		lastHash, err = item.ValueCopy(nil)

		return err
	})

	util.HandleError(err)

	chain := Chain{lastHash, db}

	return &chain
}

// FindUnspentTransactions finds all unsped transaction which belongs for address
func (c *Chain) FindUnspentTransactions() map[string]TxOutputs {
	UTXOs := make(map[string]TxOutputs)
	spentTXOs := make(map[string][]int)

	iter := c.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, stxoIdx := range spentTXOs[txID] {
						if stxoIdx == outIdx {
							continue Outputs
						}
					}
				}

				outs := UTXOs[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXOs[txID] = outs
			}

			if tx.IsCoinbase() == false {
				for _, txIn := range tx.Inputs {
					txInId := hex.EncodeToString(txIn.ID)
					spentTXOs[txInId] = append(spentTXOs[txInId], txIn.Out)
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return UTXOs
}

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

func (c *Chain) FindTransaction(ID []byte) (Transaction, error) {
	iter := c.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction does not exist")
}

func (c *Chain) SignTransaction(tx *Transaction, privateKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTX, err := c.FindTransaction(in.ID)
		util.HandleError(err)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privateKey, prevTXs)
}

func (c *Chain) VerifyTransaction(tx *Transaction) bool {
	prevTXs := make(map[string]Transaction)

	if tx.IsCoinbase() {
		return true
	}

	for _, in := range tx.Inputs {
		prevTX, err := c.FindTransaction(in.ID)

		util.HandleError(err)

		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs)
}
