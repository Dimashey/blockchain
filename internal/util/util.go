package util

import (
	"bytes"
	"encoding/binary"
	"log"

	"github.com/mr-tron/base58"
)

// ToHex converts int to hexidecimal format
func ToHex(num int64) []byte {
	buff := new(bytes.Buffer)

	err := binary.Write(buff, binary.BigEndian, num)

	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

func HandleError(err error) {
	if err != nil {
		log.Panic(err)
	}
}

func Base58Encode(input []byte) []byte {
	encode := base58.Encode(input)

	return []byte(encode)
}

func Base58Decode(input []byte) []byte {
	decode, err := base58.Decode(string(input[:]))

	if err != nil {
		log.Panic(err)
	}

	return decode
}
