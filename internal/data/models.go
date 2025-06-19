package data

import (
	"fmt"

	"math/big"
)

type Hex struct{ *big.Int }

func NewHexFromString(hex string) (*Hex, error) {
	bi := new(big.Int)
	if len(hex) >= 2 && hex[:2] == "0x" {
		hex = hex[2:]
	}
	bi, ok := bi.SetString(hex, 16)
	if !ok {
		return nil, fmt.Errorf("invalid hex string: %s", hex)
	}
	return &Hex{bi}, nil
}

func NewHexFromUint64(hex uint64) *Hex {
	return &Hex{new(big.Int).SetUint64(hex)}
}

func (h Hex) String() string {
	if h.Int == nil {
		return "0x0"
	}

	// NOTE: This is error-prone for Ethereum addresses, as leading zeroes are stripped.
	// Safe for block numbers, values, etc — but DO NOT use this for addresses.
	return fmt.Sprintf("0x%s", h.Text(16))
}
