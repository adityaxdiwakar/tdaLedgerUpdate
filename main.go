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

type QuoteEntryLastPrice struct {
	LastPrice float64 `json:"lastPrice"`
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

	/* access token update file */
	accessTokenFile := flag.String("afile", "", "TDA Access Token")

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

		if *accessTokenFile == "" {
			log.Fatalf("error: you need to provide a token file, cannot generate on the fly")
		}

		info, err := os.Stat(*accessTokenFile)
		if err != nil {
			log.Fatalf("error: could not open token file: %v\n", err)
		}

		fileTime := info.ModTime()
		dat, err := os.ReadFile(*accessTokenFile)
		if err != nil {
			log.Fatalf("error: could not open access token file: %v\n", err)
		}

		if fileTime.Before(time.Now().Add(-25*time.Minute)) || string(dat) == "" {
			fmt.Println("getting new token")
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

			if err = os.WriteFile(*accessTokenFile, []byte(accessToken), 0644); err != nil {
				log.Fatalf("error: could not write token: %v\n", err)
			}

			currentTime := time.Now().Local()
			err = os.Chtimes(*accessTokenFile, currentTime, currentTime)
			if err != nil {
				log.Fatalf("error: could not change time on access file: %v\n", err)
			}
		} else {
			fmt.Println("using old token")
			req.Header.Set("Authorization", "Bearer "+string(dat))
		}
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("error: could not get TDA response: %v\n", err)
	}

	var quotes map[string]QuoteEntryLastPrice
	json.NewDecoder(res.Body).Decode(&quotes)
	res.Body.Close()

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
				GetTimeString(), ticker, quote.LastPrice))
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
		if (e < 'A' || e > 'Z') && (e < '0' || e > '9') && e != '.' && e != '_' {
			return false
		}
	}
	return true
}
