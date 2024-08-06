package config

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-arbi-bot/internal/adapter"
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
	BUY_METHOD        = "rpc"
	SELL_METHOD       = "rpc" // "rpc", "bloxroute", "jito"
	BLOCKENGINE_URL   = "https://amsterdam.mainnet.block-engine.jito.wtf"
)

var (
	Payer                *solana.Wallet
	AddressLookupTable   solana.PublicKey
	GrpcAddr             string
	GrpcToken            string
	InsecureConnection   bool
	RedisAddr            string
	RedisPassword        string
	RpcHttpUrl           string
	RpcWsUrl             string
	BloxRouteUrl         string
	BloxRouteToken       string
	TxInterval           int
	ChunkSplitter        int64
	MachineGunMinTrigger int64
	BuyDelay             int64
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

	var err error
	txInterval, err := strconv.Atoi(os.Getenv("TX_INTERVAL"))
	if err != nil {
		txInterval = 300
	}

	TxInterval = txInterval

	mcMinTrigger, err := strconv.ParseInt(os.Getenv("MCGUN_MIN_TRIGGER"), 10, 64)
	if err != nil {
		mcMinTrigger = 500000
	}

	MachineGunMinTrigger = mcMinTrigger

	chunk, err := strconv.ParseInt(os.Getenv("CHUNK_SPLITTER"), 10, 64)
	if err != nil {
		chunk = 50
	}

	ChunkSplitter = chunk

	delay, err := strconv.ParseInt(os.Getenv("BUY_DELAY"), 10, 64)
	if err != nil {
		chunk = 3000
	}

	BuyDelay = delay

	err = adapter.InitRedisClients(RedisAddr, RedisPassword)
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
