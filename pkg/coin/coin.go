package coin

import (
	"net/http"
	"time"
	"encoding/json"
)

var APIEndpoint = "http://bittrex.com/api/v1.1/public/getmarketsummaries/"
var MarketSummaryEndpoint = "getmarketsummaries"

var myClient = &http.Client{Timeout: 10 * time.Second}

func getJson(url string, target interface{}) error {
	r, err := myClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

type MSResponse struct {
	Success bool
	Message string
	Result []MarketSummaries
}

type MarketSummaries struct {
	MarketName string
	High float64
	Low float64
	Volume float64
	Last float64
	BaseVolume float64
	TimeStamp string
	Bid float64
	Ask float64
	OpenBuyOrders float64
	OpenSellOrders float64
	PrevDay float64
	Created float64
}

func GetMarketSummary() (ms *MSResponse, err error) {
	err = getJson(APIEndpoint+MarketSummaryEndpoint,ms)
	return
}