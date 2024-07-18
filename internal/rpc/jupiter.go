package rpc

import (
	"github.com/gagliardetto/solana-go"
)

const jup = "https://lineage-ams.rpcpool.com/390cc92f-d182-4400-a829-9524d8a9e23a"

type JupiterApi struct {
}

func (*JupiterApi) fetchQuote(input solana.PublicKey, output solana.PublicKey) {

}
