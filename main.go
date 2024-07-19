package main

import (
	"errors"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/adapter"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/coder"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/config"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/generators"
	instructions "github.com/iqbalbaharum/go-solana-mev-bot/internal/instructions"
	bot "github.com/iqbalbaharum/go-solana-mev-bot/internal/library"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/liquidity"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/rpc"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

func loadAdapter() {
	adapter.GetRedisClient(0)
}

var (
	client           *pb.GeyserClient
	wsolTokenAccount solana.PublicKey
)

func main() {
	config.InitEnv()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	generators.GrpcConnect(config.GrpcAddr, config.InsecureConnection)

	txChannel := make(chan generators.GeyserResponse)

	go func() {
		for response := range txChannel {
			processResponse(response)
		}
	}()

	generators.GrpcSubscribeByAddresses(
		config.GrpcToken,
		[]string{config.RAYDIUM_AMM_V4.String()},
		[]string{}, txChannel)

	defer func() {
		if err := generators.CloseConnection(); err != nil {
			log.Printf("Error closing gRPC connection: %v", err)
		}
	}()
}

func processResponse(response generators.GeyserResponse) {
	// Your processing logic here

	c := coder.NewRaydiumAmmInstructionCoder()
	for _, ins := range response.MempoolTxns.Instructions {
		programId := response.MempoolTxns.AccountKeys[ins.ProgramIdIndex]

		if programId == config.RAYDIUM_AMM_V4.String() {
			decodedIx, err := c.Decode(ins.Data)
			if err != nil {
				// log.Println("Failed to decode instruction:", err)
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
	ammId, err := getPublicKeyFromTx(1, tx.MempoolTxns, ins)
	if err != nil {
		return
	}

	if ammId == nil {
		return
	}

	log.Print(ammId)

	pKey, err := liquidity.GetPoolKeys(ammId)
	if err != nil {
		return
	}

	reserve, err := liquidity.GetPoolSolBalance(pKey)
	if err != nil {
		return
	}

	log.Printf("%s | %d", ammId, reserve)

	// if reserve > uint64(config.LAMPORTS_PER_SOL) {
	// 	return
	// }

	log.Printf("%s | Get latest blockhash", ammId)
	blockhash, err := rpc.GetLatestBlockhash()
	if err != nil {
		log.Print(err)
		return
	}

	compute := instructions.ComputeUnit{
		MicroLamports: 0,
		Units:         0,
	}

	options := instructions.TxOption{
		Blockhash: blockhash,
	}

	log.Printf("%s | Create instructions", ammId)
	signatures, transaction, err := instructions.MakeSwapInstructions(
		pKey,
		wsolTokenAccount,
		compute,
		options,
		1000000,
		0,
		"buy",
	)

	log.Printf("%s | Send Tx %s", ammId, signatures[0])
	rpc.SendTransaction(transaction)
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
