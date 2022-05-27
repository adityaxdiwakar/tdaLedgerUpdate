package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	tda "github.com/adityaxdiwakar/tda-go"
	"github.com/google/go-querystring/query"
)

type tdaCredentials struct {
	RefreshToken string `toml:"refresh_token"`
	ConsumerKey  string `toml:"consumer_key"`
}

type QuoteEntry struct {
	AssetType                          string  `json:"assetType"`
	AssetMainType                      string  `json:"assetMainType"`
	Cusip                              string  `json:"cusip"`
	AssetSubType                       string  `json:"assetSubType"`
	Symbol                             string  `json:"symbol"`
	Description                        string  `json:"description"`
	BidPrice                           float64 `json:"bidPrice"`
	BidSize                            int     `json:"bidSize"`
	BidID                              string  `json:"bidId"`
	AskPrice                           float64 `json:"askPrice"`
	AskSize                            int     `json:"askSize"`
	AskID                              string  `json:"askId"`
	LastPrice                          float64 `json:"lastPrice"`
	LastSize                           int     `json:"lastSize"`
	LastID                             string  `json:"lastId"`
	OpenPrice                          float64 `json:"openPrice"`
	HighPrice                          float64 `json:"highPrice"`
	LowPrice                           float64 `json:"lowPrice"`
	BidTick                            string  `json:"bidTick"`
	ClosePrice                         float64 `json:"closePrice"`
	NetChange                          float64 `json:"netChange"`
	TotalVolume                        int     `json:"totalVolume"`
	QuoteTimeInLong                    int64   `json:"quoteTimeInLong"`
	TradeTimeInLong                    int64   `json:"tradeTimeInLong"`
	Mark                               float64 `json:"mark"`
	Exchange                           string  `json:"exchange"`
	ExchangeName                       string  `json:"exchangeName"`
	Marginable                         bool    `json:"marginable"`
	Shortable                          bool    `json:"shortable"`
	Volatility                         float64 `json:"volatility"`
	Digits                             int     `json:"digits"`
	Five2WkHigh                        float64 `json:"52WkHigh"`
	Five2WkLow                         float64 `json:"52WkLow"`
	NAV                                int     `json:"nAV"`
	PeRatio                            int     `json:"peRatio"`
	DivAmount                          float64 `json:"divAmount"`
	DivYield                           float64 `json:"divYield"`
	DivDate                            string  `json:"divDate"`
	SecurityStatus                     string  `json:"securityStatus"`
	RegularMarketLastPrice             float64 `json:"regularMarketLastPrice"`
	RegularMarketLastSize              int     `json:"regularMarketLastSize"`
	RegularMarketNetChange             float64 `json:"regularMarketNetChange"`
	RegularMarketTradeTimeInLong       int64   `json:"regularMarketTradeTimeInLong"`
	NetPercentChangeInDouble           float64 `json:"netPercentChangeInDouble"`
	MarkChangeInDouble                 float64 `json:"markChangeInDouble"`
	MarkPercentChangeInDouble          float64 `json:"markPercentChangeInDouble"`
	RegularMarketPercentChangeInDouble float64 `json:"regularMarketPercentChangeInDouble"`
	Delayed                            bool    `json:"delayed"`
	RealtimeEntitled                   bool    `json:"realtimeEntitled"`
}

type QuoteRequest struct {
	ApiKey  string `url:"apikey,omitempty"`
	Symbols string `url:"symbol"`
}

func main() {
	/* if neither provided, use default ./config.toml */
	configFile := flag.String("c", "./config.toml", "Use Configuration File")
	apiToken := flag.String("a", "", "TDAmeritrade App Key")

	/* get remaining parameters needed */
	ledgerBinary := flag.String("b", "ledger", "Ledger Binary")
	ledgerFile := flag.String("f", "ledger.ledger", "Ledger File")
	priceDbFile := flag.String("p", "prices.db", "Price Database File")
	flag.Parse()

	/* get commodites from ledger file */
	commodities := GetCommodities(*ledgerFile, *ledgerBinary)

	requestPayload := QuoteRequest{
		Symbols: strings.Join(commodities, ","),
	}

	if *apiToken != "" {
		requestPayload.ApiKey = *apiToken
	}

	v, err := query.Values(requestPayload)
	if err != nil {
		log.Fatalf("error: could not encode query params: %v\n", err)
	}

	url := "https://api.tdameritrade.com/v1/marketdata/quotes?" + v.Encode()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("error: could not create request for quotes: %v\n", err)
	}

	if *apiToken == "" {
		var conf tdaCredentials
		if _, err := toml.DecodeFile(*configFile, &conf); err != nil {
			log.Fatalf("error: could not parse configuration: %v\n", err)
		}

		tdaSession := tda.Session{
			Refresh:     conf.RefreshToken,
			ConsumerKey: conf.ConsumerKey,
			RootUrl:     "https://api.tdameritrade.com/v1",
		}

		accessToken, err := tdaSession.GetAccessToken()
		if err != nil {
			log.Fatalf("error: could not get access token: %v\n", err)
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("error: could not get TDA response: %v\n", err)
	}

	defer res.Body.Close()

	var quotes map[string]QuoteEntry
	json.NewDecoder(res.Body).Decode(&quotes)

	/* make file */
	pricedb, err := os.OpenFile(*priceDbFile,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		log.Fatalf("Price database file access failed with %s\n", err)
	}
	defer pricedb.Close()

	quotedTickers := make(map[string]bool)
	for ticker, quote := range quotes {
		quotedTickers[ticker] = true
		pricedb.WriteString(
			fmt.Sprintf("P %s %s $%.2f\n",
				GetTimeString(), ticker, quote.Mark))
	}

	for _, ticker := range commodities {
		if _, ok := quotedTickers[ticker]; !ok {
			fmt.Printf("Failed to quote %s\n", ticker)
		}
	}

	fmt.Println("Stock price update complete")
}

func GetTimeString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func GetCommodities(ledger string, binary string) []string {
	cmd := exec.Command(binary, "-f", ledger, "commodities")
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("error: ledger file commodity report failed with %s\n", err)
	}
	a := strings.Split(string(out), "\n")
	sliceOut := a[:len(a)-1]

	commodities := make([]string, 0)
	for _, e := range sliceOut {
		e = strings.Trim(e, `"`)
		if IsTicker(e) {
			commodities = append(commodities, e)
		}
	}
	return commodities
}

func IsTicker(s string) bool {
	for _, e := range s {
		if (e < 'A' || e > 'Z') && (e < '0' || e > '9') && e != '.' {
			return false
		}
	}
	return true
}
