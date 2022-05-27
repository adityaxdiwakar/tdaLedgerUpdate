# tdaLedgerUpdate

This application locates any stocks you have in your [ledger-cli](https://ledger-cli.org) file, then generates a price database of those stocks compatible with the application using the TDAmeritrade API. This application supports both delayed and live data depending on your authentication level.

### Usage

Build the go file, and pick an authentication method. There are two options to authenticate with the TDAmeritrade API. For delayed data, you can simply authenticate with the consumer app key and use it as follows:

```bash
./[name of executable] -f=[ledger file] -p=[price database file (to create or update)] -a=[TDAmeritade Application Key] -b=[Name of ledger binary]
```

Alternatively, you can receive live data by using full OAuth authentication. Create a file (default is `config.toml` in working directory) and add the following information:

```
refresh_token = [TDAmeritrade Refresh Token]
consumer_key = [Consumer Key]
```

Then, you can run this application using (omitting `-c` flag if using default config file location):

```bash
./[name of executable] -f=[ledger file] -p=[price database file (to create or update)] -c=[TOML File Location] -b=[Name of ledger binary]
```

This should spit out a price database file, which can then be used to calculate the market value in ledger as follows:

```bash
ledger -f [ledger file] --price-db [price database file] -V bal
```
