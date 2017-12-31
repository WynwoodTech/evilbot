package coin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var APIEndpoint = "https://bittrex.com/api/v1.1/public/"
var MarketSummaryEndpoint = "getmarketsummaries"

var GDAXEndpoing = "https://api.gdax.com/"
var GDAXTicker = "products/{PRODUCT}/ticker"

var BINANCEEndpoint = "https://api.binance.com/api/v1/"
var BINANCETicker = "ticker/24hr"

var myClient = &http.Client{Timeout: 10 * time.Second}

var msStore = map[string]MarketSummary{}

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
	Result  []MarketSummary
}

type MarketSummary struct {
	MarketName     string
	High           float64
	Low            float64
	Volume         float64
	Last           float64
	BaseVolume     float64
	TimeStamp      string
	Bid            float64
	Ask            float64
	OpenBuyOrders  float64
	OpenSellOrders float64
	PrevDay        float64
	Created        string
}

func fmtFloat64(flt float64) string {
	return strconv.FormatFloat(flt, 'f', -1, 64)
}

func StringFloat(flt string) float64 {
	f, err := strconv.ParseFloat(flt, 64)
	if err != nil {
		fmt.Printf("Error Parsing Float: %v\n", err.Error())
		fmt.Printf("Float String: %v\n", flt)
	}
	return f
}

func (ms *MarketSummary) String() string {
	markets := strings.Split(ms.MarketName, "-")
	usdMarket := markets[0] + "-USD"
	str := fmt.Sprintf(
		"MarketName: %s\n"+
			"High: %v\n"+
			"Low: %v\n"+
			"Volume: %v\n"+
			"Last: %v\n"+
			"USD: ~ %$s\n",
		ms.MarketName, fmtFloat64(ms.High), fmtFloat64(ms.Low), fmtFloat64(ms.Volume), fmtFloat64(ms.Last), GetUSDValue(ms.Last, usdMarket))
	return str
}

func GetMarketSummaryBittrex() error {
	var err error
	var ms = new(MSResponse)
	err = getJson(APIEndpoint+MarketSummaryEndpoint, ms)
	if ms.Success {
		for _, v := range ms.Result {
			log.Printf("Updating: %v\n", v.MarketName)
			msStore[v.MarketName] = v
		}
	}
	return err
}

func GetMarketSummaryBinance() error {
	var err error
	var bin []struct {
		Symbol             string `json:"symbol"`
		PriceChange        string `json:"priceChange"`
		PriceChangePercent string `json:"priceChangePercent"`
		WeightedAvgPrice   string `json:"weightedAvgPrice"`
		PrevClosePrice     string `json:"prevClosePrice"`
		LastPrice          string `json:"lastPrice"`
		LastQty            string `json:"lastQty"`
		BidPrice           string `json:"bidPrice"`
		BidQty             string `json:"bidQty"`
		AskPrice           string `json:"askPrice"`
		AskQty             string `json:"askQty"`
		OpenPrice          string `json:"openPrice"`
		HighPrice          string `json:"highPrice"`
		LowPrice           string `json:"lowPrice"`
		Volume             string `json:"volume"`
		QuoteVolume        string `json:"quoteVolume"`
		OpenTime           int    `json:"openTime"`
		CloseTime          int    `json:"closeTime"`
		FirstId            int    `json:"firstId"`
		LastId             int    `json:"lastId"`
		Count              int    `json:"count"`
	}
	err = getJson(BINANCEEndpoint+BINANCETicker, &bin)
	if err != nil {
		fmt.Printf("JSON Error: %v\n", err.Error())
		return err
	}
	for _, v := range bin {
		var marketName string
		marketName = GetMarketNameBinance([]string{"BTC", "ETH", "LTC"}, v.Symbol)
		if len(marketName) < 1 {
			continue
		}
		ms := MarketSummary{
			MarketName: marketName,
			High:       StringFloat(v.HighPrice),
			Low:        StringFloat(v.LowPrice),
			Volume:     StringFloat(v.Volume),
			Last:       StringFloat(v.LastPrice)}
		log.Printf("Updating: %v\n", marketName)
		msStore[marketName] = ms
	}
	return nil
}

func GetMarketNameBinance(market []string, s string) string {
	for _, m := range market {
		if strings.HasSuffix(s, m) {
			pref := strings.Split(s, m)
			if len(pref) > 0 {
				return m + "-" + pref[0]
			}
		}
	}
	return ""
}

func GetCurrency(market string) MarketSummary {
	return msStore[market]
}

func GetUSDValue(val float64, market string) string {
	if strings.Contains(market, "USDT"){
		return fmt.Sprintf("%.5f", val)
	}
	gdaxrs := struct {
		TradeID int `json:"trade_id"`
		Price   string
		Size    string
		Bid     string
		Ask     string
		Volume  string
		Time    string
	}{}
	ticker := strings.Replace(GDAXTicker, "{PRODUCT}", market, -1)
	fmt.Printf("Ticker: %v\n", ticker)
	err := getJson(GDAXEndpoing+ticker, &gdaxrs)
	if err != nil {
		log.Printf("Json error: %v\n", err.Error())
	}
	price, err := strconv.ParseFloat(gdaxrs.Price, 64)
	if err != nil {
		log.Printf("Cannot convert: %v\n", err.Error())
	}
	fmt.Printf("Price: %v\n", price)
	fmt.Printf("Price: %v\n", gdaxrs.Price)
	r := val * price
	return fmt.Sprintf("%.5f", r)
}

func ListMarkets() map[string]float64{
	r := map[string]float64{}
	for k,v := range msStore {
		r[k] = v.Last
	}
	return r
}
