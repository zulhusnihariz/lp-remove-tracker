package types

import "github.com/gagliardetto/solana-go"

type Trade struct {
	AmmId     *solana.PublicKey
	Mint      *solana.PublicKey
	Action    string
	Amount    string
	Signature string
	Timestamp int64
}
