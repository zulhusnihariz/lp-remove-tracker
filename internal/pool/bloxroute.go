package pool

import (
	"log"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/iqbalbaharum/lp-remove-tracker/internal/rpc"
)

type BloxRoutePoolStream struct {
	transaction   *solana.Transaction
	useStakedFlag bool
}

type BloxRoutePool struct {
	clients []*rpc.BloxRouteRpc
	taskCh  chan *BloxRoutePoolStream
	wg      sync.WaitGroup
}

func NewBloxRoutePool(numClients int) (*BloxRoutePool, error) {
	clients := make([]*rpc.BloxRouteRpc, numClients)

	for i := 0; i < numClients; i++ {
		client, err := rpc.NewBloxRouteRpc()
		if err != nil {
			return nil, err
		}
		clients[i] = client
	}

	pool := &BloxRoutePool{
		clients: clients,
		taskCh:  make(chan *BloxRoutePoolStream, 100),
	}

	for i := 0; i < numClients; i++ {
		pool.wg.Add(1)
		go pool.worker(clients[i])
	}

	return pool, nil
}

func (p *BloxRoutePool) worker(client *rpc.BloxRouteRpc) {
	defer p.wg.Done()
	for stream := range p.taskCh {
		err := client.StreamBloxRouteTransaction(stream.transaction, stream.useStakedFlag)
		if err != nil {
			log.Println("Failed to send transaction:", err)
		}
	}
}

func (p *BloxRoutePool) SendTransaction(transaction *solana.Transaction, useStakedFlag bool) {
	p.taskCh <- &BloxRoutePoolStream{
		transaction:   transaction,
		useStakedFlag: useStakedFlag,
	}
}

func (p *BloxRoutePool) Close() {
	close(p.taskCh)
	p.wg.Wait()
}
