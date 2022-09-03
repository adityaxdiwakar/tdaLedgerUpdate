package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/adityaxdiwakar/tda-go/pkg/tda"
)

var (
	ledgerFile   *string
	ledgerBinary *string
	priceDbFile  *string
	tdaSession   *tda.Session
)

type tdaCredentials struct {
	RefreshToken string `toml:"refresh_token"`
	ConsumerKey  string `toml:"consumer_key"`
}

func init() {
	/* if neither provided, use default ./config.toml */
	configFile := flag.String("c", "./config.toml", "Use Configuration File")

	/* get remaining parameters needed */
	ledgerBinary = flag.String("b", "ledger", "Ledger Binary")
	ledgerFile = flag.String("f", "ledger.ledger", "Ledger File")
	priceDbFile = flag.String("p", "prices.db", "Price Database File")

	/* access token update file */
	accessTokenFile := flag.String("afile", "", "TDA Access Token")

	flag.Parse()

	var conf tdaCredentials
	if _, err := toml.DecodeFile(*configFile, &conf); err != nil {
		log.Fatalf("error: could not parse configuration: %v\n", err)
	}

	tdaSession = tda.NewSession(
		conf.RefreshToken,
		conf.ConsumerKey,
		"https://api.tdameritrade.com/v1",
		tda.WithHttpClient(*http.DefaultClient),
		tda.WithStatPath(*accessTokenFile),
	)
}

func main() {
	/* get commodites from ledger file */
	commodities := GetCommodities(*ledgerFile, *ledgerBinary)

	quotes, err := tdaSession.GetQuotes(commodities)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	/* make file */
	pricedb, err := os.OpenFile(*priceDbFile,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		log.Fatalf("Price database file access failed with %s\n", err)
	}
	defer pricedb.Close()

	quotedTickers := make(map[string]bool)
	for ticker, quote := range *quotes {
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
