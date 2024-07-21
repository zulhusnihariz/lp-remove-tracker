package instructions

import (
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/config"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/liquidity"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/types"
)

type ComputeUnit struct {
	MicroLamports uint64
	Units         uint32
}

type TxOption struct {
	Blockhash solana.Hash
}

func MakeSwapInstructions(
	poolKeys *types.RaydiumPoolKeys,
	wsolTokenAccount solana.PublicKey,
	compute ComputeUnit,
	options TxOption,
	amountIn uint64,
	minAmountOut uint64,
	action string) ([]solana.Signature, *solana.Transaction, error) {

	var tokenAccountIn solana.PublicKey
	var tokenAccountOut solana.PublicKey

	startInstructions := []solana.Instruction{}
	computeInstructions := []solana.Instruction{}
	endInstructions := []solana.Instruction{}

	//
	_, reverse, err := liquidity.GetMint(poolKeys)
	if err != nil {
		return nil, nil, err
	}

	var accountOut solana.PublicKey

	if action == "buy" {
		if reverse {
			accountOut = poolKeys.QuoteMint
		} else {
			accountOut = poolKeys.BaseMint
		}

		ata, ins, err := createInstruction(accountOut)

		tokenAccountIn = wsolTokenAccount
		tokenAccountOut = ata

		if err != nil {
			return nil, nil, err
		}

		startInstructions = ins
	} else {

		if reverse {
			accountOut = poolKeys.QuoteMint
		} else {
			accountOut = poolKeys.BaseMint
		}

		ata, err := GetAssociatedTokenAccount(accountOut)
		if err != nil {
			return nil, nil, err
		}

		tokenAccountIn = ata
		tokenAccountOut = wsolTokenAccount
	}

	swapInstruction := MakeRaydiumSwapFixedInInstruction(&LiquiditySwapFixedInInstructionParams{
		InAmount:         amountIn,
		MinimumOutAmount: minAmountOut,
		PoolKeys:         *poolKeys,
		TokenAccountIn:   tokenAccountIn,
		TokenAccountOut:  tokenAccountOut,
		Owner:            config.Payer.PublicKey(),
	})

	if compute.Units > 0 {
		computeInstructions = append(
			computeInstructions,
			computebudget.NewSetComputeUnitLimitInstruction(compute.Units).Build())
	}

	if compute.MicroLamports > 0 {
		computeInstructions = append(
			computeInstructions,
			computebudget.NewSetComputeUnitPriceInstruction(compute.MicroLamports).Build())
	}

	ins := []solana.Instruction{}
	ins = append(ins, computeInstructions...)
	ins = append(ins, startInstructions...)
	ins = append(ins, swapInstruction)
	ins = append(ins, endInstructions...)

	alt := map[solana.PublicKey]solana.PublicKeySlice{
		config.AddressLookupTable: {
			config.AddressLookupTable,
		},
	}

	tx, err := solana.NewTransaction(
		ins,
		options.Blockhash,
		solana.TransactionPayer(config.Payer.PublicKey()),
		solana.TransactionAddressTables(alt),
	)

	if err != nil {
		return nil, nil, err
	}

	signature, err := tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if config.Payer.PublicKey().Equals(key) {
				return &config.Payer.PrivateKey
			}
			return nil
		},
	)

	if err != nil {
		return nil, nil, err
	}

	return signature, tx, nil
}

func createInstruction(mint solana.PublicKey) (solana.PublicKey, []solana.Instruction, error) {
	ins := []solana.Instruction{}

	ata, err := GetAssociatedTokenAccount(mint)
	if err != nil {
		return solana.PublicKey{}, ins, err
	}

	createInstr := associatedtokenaccount.NewCreateInstruction(
		config.Payer.PublicKey(),
		config.Payer.PublicKey(),
		mint).Build()

	ins = append(ins, createInstr)

	return ata, ins, nil
}
