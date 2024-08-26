package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"

	_ "go.uber.org/automaxprocs"

	"github.com/gagliardetto/solana-go"
	"github.com/go-chi/chi/v5"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/adapter"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/config"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/generators"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/handler"
	bot "github.com/iqbalbaharum/lp-remove-tracker/internal/library"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/storage"
)

type Server struct {
	Router *chi.Mux
}

func CreateServer() *Server {
	server := &Server{
		Router: handler.CreateRoutes(),
	}

	return server
}

const (
	PORT = 5000
)

func loadAdapter() {
	adapter.GetRedisClient(0)
}

var (
	grpcs            []*generators.GrpcClient
	latestBlockhash  string
	wsolTokenAccount solana.PublicKey
	wg               sync.WaitGroup
	txChannel        chan generators.GeyserResponse
)

func main() {
	numCPU := runtime.NumCPU() * 2
	maxProcs := runtime.GOMAXPROCS(0)
	log.Printf("Number of logical CPUs available: %d", numCPU)
	log.Printf("Number of CPUs being used: %d", maxProcs)

	runtime.GOMAXPROCS(runtime.NumCPU())

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	err := config.InitEnv()
	if err != nil {
		log.Print(err)
		return
	}

	err = adapter.InitRedisClients(config.RedisAddr, config.RedisPassword)
	if err != nil {
		log.Fatalf(fmt.Sprintf("Failed to initialize Redis clients: %v", err))
		return
	}

	err = adapter.InitMySQLClient(config.MySqlDsn)
	if err != nil {
		log.Fatalf(fmt.Sprintf("Failed to initialize SQL client: %v", err))
		return
	}

	log.Print("Initialized ENVIRONMENT successfully")

	client, err := generators.GrpcConnect(config.GRPC1.Addr, config.GRPC1.InsecureConnection)

	if err != nil {
		log.Fatalf("Error in GRPC connection: %s ", err)
		return
	}

	client2, err := generators.GrpcConnect(config.GRPC2.Addr, config.GRPC2.InsecureConnection)

	grpcs = append(grpcs, client, client2)

	if err != nil {
		log.Fatalf("Error in GRPC connection: %s ", err)
		return
	}

	txChannel = make(chan generators.GeyserResponse)

	var processed sync.Map

	// Create a worker pool
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for response := range txChannel {
				if _, exists := processed.Load(response.MempoolTxns.Signature); !exists {
					processed.Store(response.MempoolTxns.Signature, true)
					bot.ProcessResponse(response)

					time.AfterFunc(1*time.Minute, func() {
						processed.Delete(response.MempoolTxns.Signature)
					})
				}
			}
		}()
	}

	listenFor(
		grpcs[0],
		"triton",
		[]string{
			config.RAYDIUM_AMM_V4.String(),
		}, txChannel, &wg)

	listenFor(
		grpcs[1],
		"solana-tracker",
		[]string{
			config.RAYDIUM_AMM_V4.String(),
		}, txChannel, &wg)

	mySqlClient, err := adapter.GetMySQLClient()

	if err != nil {
		panic(err)
	}

	storage.Init(mySqlClient)

	server := CreateServer()
	port := fmt.Sprintf(":%d", PORT)
	fmt.Printf("server running on port%s \n", port)

	http.ListenAndServe(port, server.Router)

	wg.Wait()

	for i := 0; i < len(grpcs); i++ {
		grpc := grpcs[i]
		defer func() {

			if err := grpc.CloseConnection(); err != nil {

				log.Printf("Error closing gRPC connection: %v", err)
			}
		}()
	}
}

// Listening geyser for new addresses
func listenFor(client *generators.GrpcClient, name string, addresses []string, txChannel chan generators.GeyserResponse, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := client.GrpcSubscribeByAddresses(
			name,
			config.GrpcToken,
			addresses,
			[]string{}, txChannel)
		if err != nil {
			log.Printf("Error in first gRPC subscription: %v", err)
		}
	}()
}
