package types

import (
	"math/big"

	"github.com/gagliardetto/solana-go"
)

type Amm struct {
	ammId     *solana.PublicKey
	baseMint  *solana.PublicKey
	quoteMint *solana.PublicKey
	base      *big.Int
	quote     *big.Int
}
