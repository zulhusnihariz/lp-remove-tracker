package config

import (
	"log"
	"os"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/go-solana-mev-bot/internal/adapter"
	"github.com/joho/godotenv"
)

var (
	WRAPPED_SOL      = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	RAYDIUM_AMM_V4   = solana.MustPrivateKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
	LAMPORTS_PER_SOL = 1000000000
	TA_RENT_LAMPORTS = 2039280
	TA_SIZE          = 165
)

var (
	Payer              solana.PrivateKey
	GrpcAddr           string
	GrpcToken          string
	InsecureConnection bool
	RedisAddr          string
	RedisPassword      string
	RpcHttpUrl         string
	RpcWsUrl           string
	FlagPoolTracked    bool
)

func InitEnv() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file")
	}

	Payer = solana.PrivateKey(os.Getenv("PAYER_PRIVATE_KEY"))
	GrpcAddr = os.Getenv("GRPC_ENDPOINT")
	GrpcToken = os.Getenv("GRPC_TOKEN")
	InsecureConnection = os.Getenv("GRPC_INSECURE") == "true"
	RedisAddr = os.Getenv("REDIS_ADDR")
	RedisPassword = os.Getenv("REDIS_PASSWORD")
	RpcHttpUrl = os.Getenv("RPC_HTTP_URL")
	RpcWsUrl = os.Getenv("RPC_WS_URL")

	FlagPoolTracked = os.Getenv("FLAG_POOL_TRACKED") == "true"

	err := adapter.InitRedisClients(RedisAddr, RedisPassword)
	if err != nil {
		log.Fatalf("Failed to initialize Redis clients: %v", err)
	}
}
