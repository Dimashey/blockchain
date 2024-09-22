package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dimashey/blockchain/internal/util"
	"github.com/dgraph-io/badger"
)

const dbPath = "./tmp/blocks_%s"

const genesisData = "First Transaction from Genesis"

type Chain struct {
	LastHash []byte
	Database *badger.DB
}

func (c *Chain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := c.Database.View(func(txn *badger.Txn) error {
		if item, err := txn.Get(blockHash); err != nil {
			return errors.New("Block is not found")
		} else {
			blockData, _ := item.ValueCopy(nil)
			block = *Deserialize(blockData)
		}

		return nil
	})

	if err != nil {
		return block, err
	}

	return block, nil
}

func (c *Chain) GetBlocksHashes() [][]byte {
	var blocks [][]byte

	iter := c.Iterator()

	for {
		block := iter.Next()

		blocks = append(blocks, block.Hash)

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return blocks
}

func (c *Chain) GetBestHeight() int {
	var lastBlock Block

	err := c.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		util.HandleError(err)
		lastHash, _ := item.ValueCopy(nil)

		item, err = txn.Get(lastHash)
		util.HandleError(err)
		lastBlockData, _ := item.ValueCopy(nil)

		lastBlock = *Deserialize(lastBlockData)

		return nil
	})

	util.HandleError(err)

	return lastBlock.Height
}

func (c *Chain) MineBlock(txs []*Transaction) *Block {
	var lastHash []byte
	var lastHeight int

	for _, tx := range txs {
		if !c.VerifyTransaction(tx) {
			log.Panic("Invalid Transaction")
		}
	}

	err := c.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		util.HandleError(err)

		lastHash, err = item.ValueCopy(nil)
		util.HandleError(err)

		item, err = txn.Get(lastHash)
		util.HandleError(err)
		lastBlockValue, _ := item.ValueCopy(nil)

		lastBlock := Deserialize(lastBlockValue)
		lastHeight = lastBlock.Height

		return err
	})

	util.HandleError(err)

	newBlock := CreateBlock(txs, lastHash, lastHeight+1)

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

func (c *Chain) AddBlock(block *Block) {
	err := c.Database.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get(block.Hash); err == nil {
			return nil
		}

		blockData := block.Serialize()
		err := txn.Set(block.Hash, blockData)
		util.HandleError(err)

		item, err := txn.Get([]byte("lh"))
		util.HandleError(err)
		lastHash, _ := item.ValueCopy(nil)

		item, err = txn.Get(lastHash)
		util.HandleError(err)
		lastBlockData, _ := item.ValueCopy(nil)

		lastBlock := Deserialize(lastBlockData)

		if block.Height > lastBlock.Height {
			err = txn.Set([]byte("lh"), block.Hash)
			util.HandleError(err)
			c.LastHash = block.Hash
		}

		return nil
	})

	util.HandleError(err)
}

func DBexists(path string) bool {
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
		return false
	}

	return true
}

func InitBlockChain(address, nodeId string) *Chain {
	var lastHash []byte

	path := fmt.Sprintf(dbPath, nodeId)

	if DBexists(path) {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions(dbPath)

	db, err := openDB(path, opts)

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

func ContinueBlockChain(nodeId string) *Chain {
	path := fmt.Sprintf(dbPath, nodeId)
	if DBexists(path) == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions(dbPath)

	db, err := openDB(path, opts)

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

func retry(dir string, originalOpts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(dir, "LOCK")

	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf(`removing "LOCK: %s"`, err)
	}
	retryOpts := originalOpts
	retryOpts.Truncate = true
	db, err := badger.Open(retryOpts)

	return db, err
}

func openDB(dir string, opts badger.Options) (*badger.DB, error) {
	if db, err := badger.Open(opts); err != nil {
		if strings.Contains(err.Error(), "LOCK") {
			if db, err := retry(dir, opts); err == nil {
				log.Panicln("database unlocked, value log truncated")
				return db, nil
			}
			log.Panicln("could not unlock database: ", err)
		}

		return nil, err
	} else {
		return db, nil
	}

}
