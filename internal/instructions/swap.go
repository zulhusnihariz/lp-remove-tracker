package instructions

import (
	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/config"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/liquidity"
)

type ComputeUnit struct {
	MicroLamports uint64
	Units         uint32
}

type TxOption struct {
	Blockhash solana.Hash
}

func MakeSwapInstructions(
	poolKeys *liquidity.RaydiumPoolKeys,
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

	tx, err := solana.NewTransaction(
		ins,
		options.Blockhash,
		solana.TransactionPayer(config.Payer.PublicKey()),
	)

	if err != nil {
		return nil, nil, err
	}

	signature, err := tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if config.Payer.PublicKey().Equals(key) {
				return &config.Payer
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

	createInstr, err := system.NewCreateAccountInstruction(
		uint64(config.TA_RENT_LAMPORTS),
		uint64(config.TA_SIZE),
		solana.TokenProgramID,
		config.Payer.PublicKey(),
		ata).ValidateAndBuild()

	if err != nil {
		return solana.PublicKey{}, []solana.Instruction{}, err
	}

	ins = append(ins, createInstr)

	initInstr, err := token.NewInitializeAccountInstruction(
		ata,
		mint,
		config.Payer.PublicKey(),
		solana.SysVarRentPubkey).ValidateAndBuild()

	if err != nil {
		return solana.PublicKey{}, []solana.Instruction{}, err
	}

	ins = append(ins, initInstr)

	return ata, ins, nil
}
