package blockchain

import "github.com/Dimashey/blockchain/block"

type BlockChain struct {
	blocks []*block.Block
}

func (c *BlockChain) prevBlock() *block.Block {
	return c.blocks[len(c.blocks)-1]
}

func (c *BlockChain) AddBlock(data string) {
	prev := c.prevBlock()

	new := block.CreateBlock(data, prev.Hash)

	c.blocks = append(c.blocks, new)
}

func (c *BlockChain) Blocks() []*block.Block {
	return c.blocks
}

func InitBlockChain() *BlockChain {
	return &BlockChain{[]*block.Block{block.Genesis()}}
}
