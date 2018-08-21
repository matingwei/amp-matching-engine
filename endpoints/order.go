package endpoints

import (
	"encoding/json"
	"log"

	"github.com/Proofsuite/amp-matching-engine/errors"
	"github.com/ethereum/go-ethereum/common"

	"github.com/Proofsuite/amp-matching-engine/engine"
	"github.com/Proofsuite/amp-matching-engine/services"
	"github.com/Proofsuite/amp-matching-engine/types"
	"github.com/Proofsuite/amp-matching-engine/ws"
	"github.com/go-ozzo/ozzo-routing"
	"github.com/gorilla/websocket"
)

type orderEndpoint struct {
	orderService *services.OrderService
	engine       *engine.Resource
}

// ServeOrderResource sets up the routing of order endpoints and the corresponding handlers.
func ServeOrderResource(rg *routing.RouteGroup, orderService *services.OrderService, engine *engine.Resource) {
	e := &orderEndpoint{orderService, engine}
	rg.Get("/orders/<address>", e.get)
	ws.RegisterChannel(ws.OrderChannel, e.ws)
	engine.SubscribeEngineResponse(e.orderService.HandleEngineResponse)
}

func (e *orderEndpoint) get(c *routing.Context) error {
	addr := c.Param("address")
	if !common.IsHexAddress(addr) {
		return errors.NewAPIError(400, "Invalid Adrress", map[string]interface{}{})
	}

	address := common.HexToAddress(addr)
	orders, err := e.orderService.GetByUserAddress(address)
	if err != nil {
		return errors.NewAPIError(400, "Fetch Error", map[string]interface{}{})
	}

	return c.Write(orders)
}

// ws function handles incoming websocket messages on the order channel
func (e *orderEndpoint) ws(input interface{}, conn *websocket.Conn) {
	msg := &types.WebSocketPayload{}

	bytes, _ := json.Marshal(input)
	if err := json.Unmarshal(bytes, &msg); err != nil {
		log.Print(err)
	}

	switch msg.Type {
	case "NEW_ORDER":
		e.handleNewOrder(msg, conn)
	case "CANCEL_ORDER":
		e.handleCancelOrder(msg, conn)
	case "NEW_TRADE":
		e.handleNewTrade(msg, conn)
	default:
		log.Println("Response with error")
	}
}

// handleNewTrade handles NewTrade messages. New trade messages are transmitted to the corresponding order channel
// and received in the handleClientResponse.
func (e *orderEndpoint) handleNewTrade(msg *types.WebSocketPayload, conn *websocket.Conn) {
	hash := common.HexToHash(msg.Hash)

	ch := ws.GetOrderChannel(hash)
	if ch != nil {
		ch <- msg
	}
}

// handleNewOrder handles NewOrder message. New order messages are transmitted to the order service after being unmarshalled
func (e *orderEndpoint) handleNewOrder(msg *types.WebSocketPayload, conn *websocket.Conn) {
	ch := make(chan *types.WebSocketPayload)
	o := types.Order{}

	bytes, err := json.Marshal(msg.Data)
	if err != nil {
		log.Print(err)
		ws.SendOrderErrorMessage(conn, err.Error())
		return
	}

	err = o.UnmarshalJSON(bytes)
	if err != nil {
		log.Print(err)
		ws.SendOrderErrorMessage(conn, err.Error())
		return
	}

	o.Hash = o.ComputeHash()

	ws.RegisterOrderConnection(o.Hash, &ws.OrderConnection{Conn: conn, ReadChannel: ch})
	ws.RegisterConnectionUnsubscribeHandler(conn, ws.OrderSocketUnsubscribeHandler(o.Hash))

	err = e.orderService.NewOrder(&o)
	if err != nil {
		log.Print(err)
		ws.SendOrderErrorMessage(conn, err.Error(), o.Hash)
		return
	}

	// NOTE: I've put the connection registration here as i feel it would be preferable to
	// validate orders but this might leads to race conditions, not exactly sure.
	// Doing this allows for doing validation in the NewOrder function which seemed more
	// clean to me
}

// handleCancelOrder handles CancelOrder message.
func (e *orderEndpoint) handleCancelOrder(p *types.WebSocketPayload, conn *websocket.Conn) {
	bytes, err := json.Marshal(p.Data)
	oc := &types.OrderCancel{}

	err = oc.UnmarshalJSON(bytes)
	if err != nil {
		log.Print(err)
		ws.SendOrderErrorMessage(conn, err.Error(), oc.Hash)
	}

	ws.RegisterOrderConnection(oc.Hash, &ws.OrderConnection{Conn: conn, Active: true})
	ws.RegisterConnectionUnsubscribeHandler(
		conn,
		ws.OrderSocketUnsubscribeHandler(oc.Hash),
	)

	err = e.orderService.CancelOrder(oc)
	if err != nil {
		log.Print(err)
		ws.SendOrderErrorMessage(conn, err.Error(), oc.Hash)
		return
	}
}

// func (e *orderEndpoint) handleNewOrder(msg *types.Message, conn *websocket.Conn) {
// 	ch := make(chan *types.Message)
// 	p := types.NewOrderPayload{}
// 	bytes, err := json.Marshal(msg.Data)
// 	if err != nil {
// 		log.Printf("Error while marshalling msg data: ", err)
// 		ws.SendOrderErrorMessage(conn, err.Error())
// 		return
// 	}
// 	err = json.Unmarshal(bytes, &p)
// 	if err != nil {
// 		log.Printf("Error while unmarshalling msg data bytes: ", err)
// 		ws.SendOrderErrorMessage(conn, err.Error())
// 		return
// 	}

// 	p.Hash = p.ComputeHash()

// 	if err != nil {
// 		ws.OrderSendErrorMessage(conn, err.Error(), p.Hash)
// 		return
// 	}

// 	// having a separate payload/request might not be needed
// 	o, err := p.ToOrder()
// 	if err != nil {
// 		ws.OrderSendErrorMessage(conn, err.Error(), p.Hash)
// 		return
// 	}

// 	err = e.orderService.NewOrder(o)
// 	if err != nil {
// 		ws.OrderSendErrorMessage(conn, err.Error(), p.Hash)
// 		return
// 	}

// 	ws.RegisterOrderConnection(p.Hash, &ws.OrderConn{Conn: conn, ReadChannel: ch})
// 	ws.RegisterConnectionUnsubscribeHandler(
// 		conn,
// 		ws.OrderSocketUnsubscribeHandler(p.Hash),
// 	)
// }
