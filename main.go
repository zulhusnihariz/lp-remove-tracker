package main

import (
	"errors"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/adapter"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/coder"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/config"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/generators"
	bot "github.com/iqbalbaharum/go-solana-mev-bot/internal/library"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/liquidity"
)

func loadAdapter() {
	adapter.GetRedisClient(0)
}

func main() {
	config.InitEnv()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	conn := generators.GrpcConnect(config.GrpcAddr, config.InsecureConnection)
	defer conn.Close()

	txChannel := make(chan generators.GeyserResponse)

	go func() {
		for response := range txChannel {
			// Process the response here
			processResponse(response)
		}
	}()

	generators.GrpcSubscribeByAddresses(
		conn,
		config.GrpcToken,
		[]string{config.RAYDIUM_AMM_V4.String()},
		[]string{}, txChannel)
}

func processResponse(response generators.GeyserResponse) {
	// Your processing logic here

	c := coder.NewRaydiumAmmInstructionCoder()
	for _, ins := range response.MempoolTxns.Instructions {
		programId := response.MempoolTxns.AccountKeys[ins.ProgramIdIndex]

		if programId == config.RAYDIUM_AMM_V4.String() {
			decodedIx, err := c.Decode(ins.Data)
			if err != nil {
				log.Println("Failed to decode instruction:", err)
				continue
			}

			switch decodedIx.(type) {
			case coder.Initialize2:
			case coder.Withdraw:
				log.Println("Withdraw", response.MempoolTxns.Signature)
				processWithdraw(ins, response)
			case coder.SwapBaseIn:
				processSwapBaseIn(ins, response)
			case coder.SwapBaseOut:
			default:
				log.Println("Unknown instruction type")
			}
		}
	}
}

func getPublicKeyFromTx(pos int, tx generators.MempoolTxn, instruction generators.TxInstruction) (*solana.PublicKey, error) {
	accountIndexes := instruction.Accounts
	if len(accountIndexes) == 0 {
		return nil, errors.New("no account indexes provided")
	}

	lookupsForAccountKeyIndex := bot.GenerateTableLookup(tx.AddressTableLookups)
	var ammId *solana.PublicKey
	accountIndex := int(accountIndexes[pos])

	if accountIndex >= len(tx.AccountKeys) {
		lookupIndex := accountIndex - len(tx.AccountKeys)
		lookup := lookupsForAccountKeyIndex[lookupIndex]
		table, err := bot.GetLookupTable(solana.MustPublicKeyFromBase58(lookup.LookupTableKey))
		if err != nil {
			return nil, err
		}

		if int(lookup.LookupTableIndex) >= len(table.Addresses) {
			return nil, errors.New("lookup table index out of range")
		}

		ammId = &table.Addresses[lookup.LookupTableIndex]

	} else {
		key := solana.MustPublicKeyFromBase58(tx.AccountKeys[accountIndex])
		ammId = &key
	}

	return ammId, nil
}

func processWithdraw(ins generators.TxInstruction, tx generators.GeyserResponse) {
	ammId, err := getPublicKeyFromTx(0, tx.MempoolTxns, ins)
	if err != nil {
		return
	}

	if ammId == nil {
		return
	}

	pKey, err := liquidity.GetPoolKeys(ammId)
	if err != nil {
		return
	}

	reserve, err := liquidity.GetPoolSolBalance(pKey)
	if err != nil {
		return
	}

	if reserve > uint64(config.LAMPORTS_PER_SOL) {
		return
	}

	// Buy
}

func processSwapBaseIn(ins generators.TxInstruction, tx generators.GeyserResponse) {
	// mint, _, err := liquidity.GetMint(pKey)
	// if err != nil {
	// 	return
	// }

	// log.Print(data.BaseMint)
}

func processBuy() {}

func processSell() {}
