package pos

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type TerminalClient struct {
	InvoiceID string
	Conn      *websocket.Conn
	Send      chan []byte
}

type Hub struct {
	// Registered terminals listening for invoice payments: map[invoiceID]*TerminalClient
	clients    map[string]*TerminalClient
	register   chan *TerminalClient
	unregister chan *TerminalClient
	broadcast  chan PaymentEvent
	mu         sync.RWMutex
}

type PaymentEvent struct {
	InvoiceID   string `json:"invoice_id"`
	TxID        string `json:"tx_id"`
	AmountUnits int64  `json:"amount_units"`
	Status      string `json:"status"`
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*TerminalClient),
		register:   make(chan *TerminalClient),
		unregister: make(chan *TerminalClient),
		broadcast:  make(chan PaymentEvent, 100),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.InvoiceID] = client
			h.mu.Unlock()
			log.Printf("🔌 Terminal connected to invoice stream: %s", client.InvoiceID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.InvoiceID]; ok {
				delete(h.clients, client.InvoiceID)
				close(client.Send)
				log.Printf("🔌 Terminal disconnected from invoice stream: %s", client.InvoiceID)
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.mu.RLock()
			client, ok := h.clients[event.InvoiceID]
			h.mu.RUnlock()

			if ok {
				msg := []byte(`{"event":"PAYMENT_CONFIRMED","invoice_id":"` + event.InvoiceID + `","status":"PAID"}`)
				select {
				case client.Send <- msg:
					log.Printf("⚡ Sub-second push sent to terminal for invoice: %s", event.InvoiceID)
				default:
					close(client.Send)
					h.mu.Lock()
					delete(h.clients, event.InvoiceID)
					h.mu.Unlock()
				}
			}
		}
	}
}

func (h *Hub) BroadcastPayment(invoiceID, txID string, amountUnits int64) {
	h.broadcast <- PaymentEvent{
		InvoiceID:   invoiceID,
		TxID:        txID,
		AmountUnits: amountUnits,
		Status:      "PAID",
	}
}

func (h *Hub) RegisterClient(client *TerminalClient) {
	h.register <- client
}

func (h *Hub) UnregisterClient(client *TerminalClient) {
	h.unregister <- client
}