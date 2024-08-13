package rpc

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/iqbalbaharum/go-arbi-bot/internal/config"
	"github.com/iqbalbaharum/go-arbi-bot/internal/generators"
)

type SlotNotification struct {
	Slot   uint64
	Parent uint64
	Root   uint64
}

type WsRpc struct {
	wsClient *generators.WSClient
	mutex    sync.Mutex
}

func NewWsRpc() (*WsRpc, error) {

	wsClient, err := generators.NewWSClient(config.RpcWsUrl, "")
	if err != nil {
		return nil, err
	}

	return &WsRpc{
		wsClient: wsClient,
	}, nil
}

func (w *WsRpc) SubscribeToSlot(slotChan chan<- SlotNotification) {
	subscriptionRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "slotSubscribe",
	}

	requestData, err := json.Marshal(subscriptionRequest)
	if err != nil {
		log.Println("Failed to marshal subscription request:", err)
		return
	}

	w.mutex.Lock()
	err = w.wsClient.SendMessage(string(requestData))
	w.mutex.Unlock()
	if err != nil {
		log.Println("Failed to send subscription request:", err)
		return
	}

	go func() {

		messageChan := make(chan []byte)

		go w.wsClient.ReadMessages(messageChan)

		for message := range messageChan {
			var response map[string]interface{}
			if err := json.Unmarshal(message, &response); err != nil {
				log.Println("Failed to unmarshal message:", err)
				continue
			}

			if response["method"] == "slotNotification" {
				if params, ok := response["params"].(map[string]interface{}); ok {
					if result, ok := params["result"].(map[string]interface{}); ok {
						slot := uint64(result["slot"].(float64))
						parent := uint64(result["parent"].(float64))
						root := uint64(result["root"].(float64))

						slotChan <- SlotNotification{
							Slot:   slot,
							Parent: parent,
							Root:   root,
						}
					}
				}
			}
		}
	}()
}
