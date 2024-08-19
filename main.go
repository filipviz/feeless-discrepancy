package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

var (
	FEE       = big.NewInt(25_000_000) // 2.5%
	MAX_FEE   = big.NewInt(1_000_000_000)
	TOLERANCE = big.NewInt(2) // 2 wei tolerance to account for apparent rounding issues.
)

func main() {
	if _, err := os.Stat(".env"); err == nil {
		godotenv.Load()
	} else {
		log.Fatalln("Could not find .env file. See .example.env.")
	}

	endpoint := os.Getenv("SUBGRAPH_ENDPOINT")
	if endpoint == "" {
		log.Fatalln("Could not find SUBGRAPH_ENDPOINT in environment. See .example.env.")
	}

	// No need to paginate at the time of writing – fewer than 1,000 distributions in v2/3.
	queryBody := struct {
		Query string `json:"query"`
	}{
		Query: `{
  distributePayoutsEvents(
    first: 1000
    skip: 0
    orderBy: timestamp
    where: {splitDistributions_: {splitProjectId_not: 0}}
  ) {
    amount
    fee
    projectId
    beneficiaryDistributionAmount
    beneficiary
    splitDistributions {
      amount
      beneficiary
      splitProjectId
    }
    txHash
  }
}`,
	}

	jsonBody, err := json.Marshal(queryBody)
	if err != nil {
		log.Fatalln("Error marshalling query:", err)
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Fatalln("Error making request:", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("Error reading response body:", err)
	}

	var response GraphQLResponse
	if err := json.Unmarshal(body, &response); err != nil {
		log.Fatalln("Error unmarshalling response:", err)
	}

	log.Printf("Processing %d transactions", len(response.Data.DistributePayoutsEvents))

	type discrepancy struct {
		amount *big.Int
		count  int
	}
	discrepancyFor := make(map[int]discrepancy)

	for _, evt := range response.Data.DistributePayoutsEvents {
		// Start with the beneficiary distribution amount.
		amountToTakeFeeFrom := evt.BeneficiaryDistributionAmount

		// Iterate through the splits.
		for _, split := range evt.SplitDistributions {
			// If the beneficiary is an address, add it to the amount to take fees from.
			if split.SplitProjectID == 0 {
				amountToTakeFeeFrom.Add(amountToTakeFeeFrom, split.Amount)
			}
		}

		// Calculate the expected fee. amount to take fees from * 2.5% / 100%
		expectedFee := new(big.Int).Mul(amountToTakeFeeFrom, FEE)
		expectedFee.Div(expectedFee, MAX_FEE)

		difference := new(big.Int).Sub(evt.Fee, expectedFee)

		// If the difference exceeds the tolerance, update the discrepancy map.
		if difference.Cmp(TOLERANCE) > 0 {
			current, ok := discrepancyFor[evt.ProjectID]
			if !ok {
				current = discrepancy{
					amount: big.NewInt(0),
					count:  0,
				}
			}
			current.amount.Add(current.amount, difference)
			current.count++
			discrepancyFor[evt.ProjectID] = current
		}
	}

	// Log the discrepancies we recorded.
	for projectID, discrepancy := range discrepancyFor {
		log.Printf("Project %d has a discrepancy of %s across %d transactions.\n",
			projectID, discrepancy.amount, discrepancy.count)
	}
}
