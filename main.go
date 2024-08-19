package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

var (
	SPLITS_TOTAL_PERCENT = big.NewInt(1_000_000_000)
	endpoint             = ""
)

func main() {
	if _, err := os.Stat(".env"); err == nil {
		godotenv.Load()
	} else {
		log.Fatalln("Could not find .env file. See .example.env.")
	}

	endpoint = os.Getenv("SUBGRAPH_ENDPOINT")
	if endpoint == "" {
		log.Fatalln("Could not find SUBGRAPH_ENDPOINT in environment. See .example.env.")
	}

	// Fetch the times JuiceboxDAO's payouts have been distributed.
	resp, err := subgraphQuery(`{
	  distributePayoutsEvents(
	    first: 1000
	    where: {projectId: 1}
	  ) {
	    distributedAmount
	    splitDistributions {
	      percent
	      splitProjectId
	    }
	    txHash
	  }
	}`)
	if err != nil {
		log.Fatalf("Error querying subgraph: %v", err)
	}

	var jbdaoPayouts GraphQLResponse
	if err = json.Unmarshal(resp, &jbdaoPayouts); err != nil {
		log.Fatalf("Error unmarshalling response: %v", err)
	}

	// A struct to hold discrepancy details.
	type Discrepancy struct {
		discrepancyAmount *big.Int
		expectedAmount    *big.Int
		amountReceived    *big.Int
		txHash            string
	}
	discrepancies := make(map[int][]Discrepancy)

	// Iterate through each payout distribution.
	for _, payout := range jbdaoPayouts.Data.DistributePayoutsEvents {
		// Build a map of the expected amounts each project should receive.
		expectedAmounts := make(map[int]*big.Int)
		for _, split := range payout.SplitDistributions {
			// Skip payouts to wallets.
			if split.SplitProjectID == 0 {
				continue
			}

			// Calculate the expected amount for the project.
			expectedAmount := big.NewInt(split.Percent)
			expectedAmount.Mul(expectedAmount, payout.DistributedAmount)
			expectedAmount.Div(expectedAmount, SPLITS_TOTAL_PERCENT)
			expectedAmounts[split.SplitProjectID] = expectedAmount
		}

		// Query the payEvents which took place in the same transaction.
		query := fmt.Sprintf(`{
		  payEvents(
		    first: 1000
		    where: {txHash: "%s"}
		  ) {
		    projectId
		    amount
		  }
		}`, payout.TxHash)
		resp, err := subgraphQuery(query)
		if err != nil {
			log.Fatalf("Error querying subgraph: %v", err)
		}
		var payEvents GraphQLResponse
		if err = json.Unmarshal(resp, &payEvents); err != nil {
			log.Fatalf("Error unmarshalling response: %v", err)
		}

		// Iterate through each pay event to check for discrepancies.
		for _, evt := range payEvents.Data.PayEvents {
			// Skip payouts to JuiceboxDAO (fees).
			if evt.ProjectID == 1 {
				continue
			}

			expectedAmount, ok := expectedAmounts[evt.ProjectID]
			if !ok {
				log.Printf("Project %d not found in expected amounts", evt.ProjectID)
			}

			// Compare the expected amount to the amount received.
			cmp := expectedAmount.Cmp(evt.Amount)
			if cmp > 0 {
				// The project was underpaid (a discrepancy).
				difference := new(big.Int).Sub(expectedAmount, evt.Amount)

				// Initialize the slice for this project if it doesn't exist.
				if _, ok := discrepancies[evt.ProjectID]; !ok {
					discrepancies[evt.ProjectID] = []Discrepancy{}
				}

				// Add the discrepancy to the slice.
				discrepancies[evt.ProjectID] = append(discrepancies[evt.ProjectID], Discrepancy{
					discrepancyAmount: difference,
					expectedAmount:    expectedAmount,
					amountReceived:    evt.Amount,
					txHash:            payout.TxHash,
				})
			} else if cmp < 0 {
				// The project was overpaid. Log it (but don't include it in the report).
				difference := new(big.Int).Sub(evt.Amount, expectedAmount)
				log.Printf("Project %d was overpaid by %s in tx %s", evt.ProjectID, weiToEth(difference), payout.TxHash)
			}
		}
	}

	// Create the report file.
	report, err := os.Create("report.md")
	if err != nil {
		log.Fatalf("Error creating report file: %v", err)
	}
	defer report.Close()

	for projectID, discrepancies := range discrepancies {
		// Calculate the total amount owed to the project.
		total := big.NewInt(0)
		for _, discrepancy := range discrepancies {
			total.Add(total, discrepancy.discrepancyAmount)
		}

		report.WriteString(fmt.Sprintf("## Project #%d\n\n[Link to project](https://juicebox.money/v2/p/%d).\n\n",
			projectID, projectID))
		report.WriteString(fmt.Sprintf("Underpaid by %s. Breakdown:\n\n", weiToEth(total)))
		report.WriteString("| Underpaid by | Expected | Received | Tx Hash |\n| --- | --- | --- | --- |\n")
		// Write the discrepancies to the report table.
		for _, discrepancy := range discrepancies {
			report.WriteString(fmt.Sprintf("| %s | %s | %s | [`%s`](https://etherscan.io/tx/%s) |\n",
				weiToEth(discrepancy.discrepancyAmount), weiToEth(discrepancy.expectedAmount),
				weiToEth(discrepancy.amountReceived), discrepancy.txHash, discrepancy.txHash))
		}
		report.WriteString("\n\n")
	}
}

// Query the subgraph and return the response body as a bytes slice.
func subgraphQuery(query string) ([]byte, error) {
	queryBody := struct {
		Query string `json:"query"`
	}{
		Query: query,
	}

	jsonBody, err := json.Marshal(queryBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling query: %w", err)
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}

// Convert wei big.Int to a formatted ETH string with full precision.
func weiToEth(wei *big.Int) string {
	weiFloat := new(big.Float).SetInt(wei)
	weiPerEth := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	ethFloat := new(big.Float).Quo(weiFloat, weiPerEth)
	return ethFloat.Text('f', -1) + " ETH"
}
