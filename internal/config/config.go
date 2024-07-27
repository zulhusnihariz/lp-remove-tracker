package config

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/adapter"
	"github.com/joho/godotenv"
)

var (
	WRAPPED_SOL       = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	RAYDIUM_AMM_V4    = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
	OPENBOOK_ID       = solana.MustPublicKeyFromBase58("srmqPvymJeFKQ4zGQed1GFppgkRHL9kaELCbyksJtPX")
	RAYDIUM_AUTHORITY = solana.MustPublicKeyFromBase58("5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1")
	BLOXROUTE_MEMO    = solana.MustPublicKeyFromBase58("HQ2UUt18uJqKaQFJhgV9zaTdQxUZjNrsKFgoEDquBkcx")
	BLOXROUTE_TIP     = solana.MustPublicKeyFromBase58("HWEoBxYs7ssKuudEjzjmpfJVX7Dvi7wescFsVx2L5yoY")
	LAMPORTS_PER_SOL  = 1000000000
	TA_RENT_LAMPORTS  = 2039280
	TA_SIZE           = 165
	SELL_METHOD       = "bloxkroute" // "rpc", "bloxroute", "jito"
	BLOCKENGINE_URL   = "https://amsterdam.mainnet.block-engine.jito.wtf"
)

var (
	Payer              *solana.Wallet
	AddressLookupTable solana.PublicKey
	GrpcAddr           string
	GrpcToken          string
	InsecureConnection bool
	RedisAddr          string
	RedisPassword      string
	RpcHttpUrl         string
	RpcWsUrl           string
	BloxRouteUrl       string
	BloxRouteToken     string
)

func InitEnv() error {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file")
	}

	pay, e := solana.WalletFromPrivateKeyBase58(os.Getenv("PAYER_PRIVATE_KEY"))
	if e != nil {
		return e
	}

	Payer = pay

	AddressLookupTable = solana.MustPublicKeyFromBase58(os.Getenv("RAYDIUM_ALT"))
	GrpcAddr = os.Getenv("GRPC_ENDPOINT")
	GrpcToken = os.Getenv("GRPC_TOKEN")
	InsecureConnection = os.Getenv("GRPC_INSECURE") == "true"
	RedisAddr = os.Getenv("REDIS_ADDR")
	RedisPassword = os.Getenv("REDIS_PASSWORD")
	RpcHttpUrl = os.Getenv("RPC_HTTP_URL")
	RpcWsUrl = os.Getenv("RPC_WS_URL")
	BloxRouteUrl = os.Getenv("BLOXROUTE_URL")
	BloxRouteToken = os.Getenv("BLOXROUTE_TOKEN")

	err := adapter.InitRedisClients(RedisAddr, RedisPassword)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to initialize Redis clients: %v", err))
	}

	return nil
}

func GetJitoTipAddress() solana.PublicKey {

	var mainnetTipAccounts = []solana.PublicKey{
		solana.MustPublicKeyFromBase58("96gYZGLnJYVFmbjzopPSU6QiEV5fGqZNyN9nmNhvrZU5"),
		solana.MustPublicKeyFromBase58("HFqU5x63VTqvQss8hp11i4wVV8bD44PvwucfZ2bU7gRe"),
		solana.MustPublicKeyFromBase58("Cw8CFyM9FkoMi7K7Crf6HNQqf4uEMzpKw6QNghXLvLkY"),
		solana.MustPublicKeyFromBase58("ADaUMid9yfUytqMBgopwjb2DTLSokTSzL1zt6iGPaS49"),
		solana.MustPublicKeyFromBase58("DfXygSm4jCyNCybVYYK6DwvWqjKee8pbDmJGcLWNDXjh"),
		solana.MustPublicKeyFromBase58("ADuUkR4vqLUMWXxW9gh6D6L8pMSawimctcNZ5pGwDcEt"),
		solana.MustPublicKeyFromBase58("DttWaMuVvTiduZRnguLF7jNxTgiMBZ1hyAumKUiL2KRL"),
		solana.MustPublicKeyFromBase58("3AVi9Tg9Uo68tJfuvoKvqKNWKkC5wPdSSdeBnizKZ6jT"),
	}

	randomIndex := rand.Intn(len(mainnetTipAccounts))
	return mainnetTipAccounts[randomIndex]
}
