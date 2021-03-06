package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Proofsuite/amp-matching-engine/daos"
	"github.com/Proofsuite/amp-matching-engine/types"
	"github.com/Proofsuite/amp-matching-engine/utils"
	"github.com/Proofsuite/amp-matching-engine/ws"
	"github.com/gorilla/websocket"
	"gopkg.in/mgo.v2/bson"

	"github.com/ethereum/go-ethereum/common"
)

type OHLCVService struct {
	tradeDao *daos.TradeDao
}

func NewOHLCVService(TradeDao *daos.TradeDao) *OHLCVService {
	return &OHLCVService{TradeDao}
}

// UnregisterForTicks handles all the unsubscription messages for ticks corresponding to a pair
func (s *OHLCVService) Unsubscribe(conn *websocket.Conn, bt, qt common.Address, params *types.Params) {
	id := utils.GetOHLCVChannelID(bt, qt, params.Units, params.Duration)
	ws.GetTradeSocket().Unsubscribe(id, conn)
}

// RegisterForTicks handles all the subscription messages for ticks corresponding to a pair
// It calls the corresponding channel's subscription method and sends trade history back on the connection
func (s *OHLCVService) Subscribe(conn *websocket.Conn, bt, qt common.Address, params *types.Params) {
	ohlcv, err := s.GetOHLCV([]types.PairSubDoc{types.PairSubDoc{BaseToken: bt, QuoteToken: qt}},
		params.Duration,
		params.Units,
		params.From,
		params.To,
	)

	if err != nil {
		ws.SendTradeErrorMessage(conn, err.Error())
	}

	id := utils.GetOHLCVChannelID(bt, qt, params.Units, params.Duration)
	err = ws.GetTradeSocket().Subscribe(id, conn)
	if err != nil {
		message := map[string]string{
			"Code":    "UNABLE_TO_SUBSCRIBE",
			"Message": "UNABLE_TO_SUBSCRIBE: " + err.Error(),
		}

		ws.SendTradeErrorMessage(conn, message)
	}

	ws.RegisterConnectionUnsubscribeHandler(conn, ws.GetTradeSocket().UnsubscribeHandler(id))
	ws.SendTradeInitMessage(conn, ohlcv)
}

// GETOHLCV fetches OHLCV data using
// pairName: can be "" for fetching data for all pairs
// duration: in integer
// unit: sec,min,hour,day,week,month,yr
// timeInterval: 0-2 entries (0 argument: latest data,1st argument: from timestamp, 2nd argument: to timestamp)
func (s *OHLCVService) GetOHLCV(pairs []types.PairSubDoc, duration int64, unit string, timeInterval ...int64) ([]*types.Tick, error) {
	match := bson.M{}
	addFields := bson.M{}
	resp := []*types.Tick{}

	currentTs := time.Now().UnixNano() / int64(time.Second)
	sort := bson.M{"$sort": bson.M{"createdAt": 1}}
	group := bson.M{
		"count": bson.M{"$sum": 1},
		"h":     bson.M{"$max": "$price"},
		"l":     bson.M{"$min": "$price"},
		"o":     bson.M{"$first": "$price"},
		"c":     bson.M{"$last": "$price"},
		"v":     bson.M{"$sum": "$amount"},
	}

	var intervalSeconds int64
	var modTime int64
	switch unit {
	case "sec":
		group["_id"], addFields = getGroupTsBson("$createdAt", "sec", duration)
		intervalSeconds = duration
		modTime = currentTs - int64(math.Mod(float64(currentTs), float64(intervalSeconds)))

	case "hour":
		group["_id"], addFields = getGroupTsBson("$createdAt", "hour", duration)
		intervalSeconds = duration * 60 * 60
		modTime = currentTs - int64(math.Mod(float64(currentTs), float64(intervalSeconds)))

	case "day":
		group["_id"], addFields = getGroupTsBson("$createdAt", "day", duration)
		intervalSeconds = duration * 24 * 60 * 60
		modTime = currentTs - int64(math.Mod(float64(currentTs), float64(intervalSeconds)))

	case "week":
		group["_id"], addFields = getGroupTsBson("$createdAt", "week", duration)
		intervalSeconds = duration * 7 * 24 * 60 * 60
		modTime = currentTs - int64(math.Mod(float64(currentTs), float64(intervalSeconds)))

	case "month":
		group["_id"], addFields = getGroupTsBson("$createdAt", "month", duration)
		d := time.Date(time.Now().Year(), time.Now().Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		intervalSeconds = duration * int64(d) * 7 * 24 * 60 * 60
		modTime = currentTs - int64(math.Mod(float64(currentTs), float64(intervalSeconds)))

	case "yr":
		group["_id"], addFields = getGroupTsBson("$createdAt", "yr", duration)
		// Number of days in current year
		d := time.Date(time.Now().Year()+1, 0, 0, 0, 0, 0, 0, time.UTC).Sub(time.Date(time.Now().Year(), 0, 0, 0, 0, 0, 0, time.UTC)).Hours() / 24

		intervalSeconds = duration * int64(d) * 7 * 24 * 60 * 60
		modTime = currentTs - int64(math.Mod(float64(currentTs), float64(intervalSeconds)))

	case "":
	case "min":
		group["_id"], addFields = getGroupTsBson("$createdAt", "min", duration)
		intervalSeconds = duration * 60
		modTime = currentTs - int64(math.Mod(float64(currentTs), float64(intervalSeconds)))

	default:
		return nil, errors.New("Invalid unit")
	}

	lt := time.Unix(modTime, 0)
	gt := time.Unix(modTime-intervalSeconds, 0)

	if len(timeInterval) == 0 {
		match = bson.M{"createdAt": bson.M{"$lt": lt}}
	} else if len(timeInterval) >= 1 {
		lt = time.Unix(timeInterval[1], 0)
		gt = time.Unix(timeInterval[0], 0)
		match = bson.M{"createdAt": bson.M{"$gte": gt, "$lt": lt}}
	}

	if len(pairs) >= 1 {
		or := make([]bson.M, 0)

		for _, pair := range pairs {
			or = append(or, bson.M{
				"$and": []bson.M{
					bson.M{
						"baseToken":  pair.BaseToken.Hex(),
						"quoteToken": pair.QuoteToken.Hex(),
					},
				},
			},
			)
		}

		match["$or"] = or
		fmt.Println(or)
	}

	match = bson.M{"$match": match}
	group = bson.M{"$group": group}
	query := []bson.M{match, sort, group, addFields, bson.M{"$sort": bson.M{"ts": 1}}}
	aggregateResp, err := s.tradeDao.Aggregate(query)

	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(aggregateResp)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(bytes, &resp)
	return resp, nil
}

// query for grouping of the documents and addition of required fields using aggregate pipeline
func getGroupTsBson(key, units string, duration int64) (resp bson.M, addFields bson.M) {
	t := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	var d interface{}
	if key == "now" {
		d = time.Now()
	} else {
		d = key
	}

	if units == "sec" {
		resp = bson.M{
			"year":   bson.M{"$year": d},
			"day":    bson.M{"$dayOfMonth": d},
			"month":  bson.M{"$month": d},
			"hour":   bson.M{"$hour": d},
			"minute": bson.M{"$minute": d},
			"second": bson.M{
				"$subtract": []interface{}{
					bson.M{"$second": d},
					bson.M{"$mod": []interface{}{bson.M{"$second": d}, duration}},
				},
			},
		}

		addFields = bson.M{"$addFields": bson.M{"ts": bson.M{"$subtract": []interface{}{bson.M{"$dateFromParts": bson.M{
			"year":   "$_id.year",
			"month":  "$_id.month",
			"day":    "$_id.day",
			"hour":   "$_id.hour",
			"minute": "$_id.minute",
			"second": "$_id.second",
		},
		}, t,
		},
		},
		},
		}
	} else if units == "min" {
		resp = bson.M{
			"year":  bson.M{"$year": d},
			"day":   bson.M{"$dayOfMonth": d},
			"month": bson.M{"$month": d},
			"hour":  bson.M{"$hour": d},
			"minute": bson.M{
				"$subtract": []interface{}{
					bson.M{"$minute": d},
					bson.M{"$mod": []interface{}{bson.M{"$minute": d}, duration}},
				},
			},
		}

		addFields = bson.M{"$addFields": bson.M{"ts": bson.M{"$subtract": []interface{}{bson.M{"$dateFromParts": bson.M{
			"year":   "$_id.year",
			"month":  "$_id.month",
			"day":    "$_id.day",
			"hour":   "$_id.hour",
			"minute": "$_id.minute",
		}}, t}}}}

	} else if units == "hour" {
		resp = bson.M{
			"year":  bson.M{"$year": d},
			"day":   bson.M{"$dayOfMonth": d},
			"month": bson.M{"$month": d},
			"hour": bson.M{
				"$subtract": []interface{}{
					bson.M{"$hour": d},
					bson.M{"$mod": []interface{}{bson.M{"$hour": d}, duration}},
				},
			},
		}

		addFields = bson.M{"$addFields": bson.M{"ts": bson.M{"$subtract": []interface{}{bson.M{"$dateFromParts": bson.M{
			"year":  "$_id.year",
			"month": "$_id.month",
			"day":   "$_id.day",
			"hour":  "$_id.hour",
		}}, t}}}}

	} else if units == "day" {
		resp = bson.M{
			"year":  bson.M{"$year": d},
			"month": bson.M{"$month": d},
			"day": bson.M{
				"$subtract": []interface{}{
					bson.M{"$dayOfMonth": d},
					bson.M{"$mod": []interface{}{bson.M{"$dayOfMonth": d}, duration}},
				},
			},
		}

		addFields = bson.M{"$addFields": bson.M{"ts": bson.M{"$subtract": []interface{}{bson.M{"$dateFromParts": bson.M{
			"year":  "$_id.year",
			"month": "$_id.month",
			"day":   "$_id.day",
		}}, t}}}}

	} else if units == "week" {
		resp = bson.M{
			"year": bson.M{"$year": d},
			"isoWeek": bson.M{
				"$subtract": []interface{}{
					bson.M{"$isoWeek": d},
					bson.M{"$mod": []interface{}{bson.M{"$isoWeek": d}, duration}},
				},
			},
		}

		addFields = bson.M{"$addFields": bson.M{"ts": bson.M{"$subtract": []interface{}{bson.M{"$dateFromParts": bson.M{
			"year":    "$_id.year",
			"isoWeek": "$_id.isoWeek",
		}}, t}}}}

	} else if units == "month" {
		resp = bson.M{
			"year": bson.M{"$year": d},
			"month": bson.M{
				"$subtract": []interface{}{
					bson.M{"$month": d},
					bson.M{"$mod": []interface{}{bson.M{"$month": d}, duration}},
				},
			},
		}

		addFields = bson.M{"$addFields": bson.M{"ts": bson.M{"$subtract": []interface{}{bson.M{"$dateFromParts": bson.M{
			"year":  "$_id.year",
			"month": "$_id.month",
		}}, t}}}}
	} else if units == "yr" {
		resp = bson.M{
			"year": bson.M{
				"$subtract": []interface{}{
					bson.M{"$year": d},
					bson.M{"$mod": []interface{}{bson.M{"$year": d}, duration}},
				},
			},
		}

		addFields = bson.M{"$addFields": bson.M{"ts": bson.M{"$subtract": []interface{}{bson.M{"$dateFromParts": bson.M{
			"year": "$_id.year"}}, t}}}}
	}
	resp["pair"] = "$pairName"
	resp["baseToken"] = "$baseToken"
	resp["quoteToken"] = "$quoteToken"
	return
}
