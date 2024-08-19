package main

import (
	"encoding/json"
	"fmt"
	"math/big"
)

type GraphQLResponse struct {
	Data Data `json:"data"`
}

type Data struct {
	DistributePayoutsEvents []DistributePayoutsEvent `json:"distributePayoutsEvents"`
}

type DistributePayoutsEvent struct {
	Amount                        *big.Int            `json:"amount"`
	Fee                           *big.Int            `json:"fee"`
	ProjectID                     int                 `json:"projectId"`
	BeneficiaryDistributionAmount *big.Int            `json:"beneficiaryDistributionAmount"`
	Beneficiary                   string              `json:"beneficiary"`
	SplitDistributions            []SplitDistribution `json:"splitDistributions"`
	TxHash                        string              `json:"txHash"`
}

type SplitDistribution struct {
	Amount         *big.Int `json:"amount"`
	Beneficiary    string   `json:"beneficiary"`
	SplitProjectID int      `json:"splitProjectId"`
}

// Custom UnmarshalJSON for DistributePayoutsEvent to handle big.Int
func (d *DistributePayoutsEvent) UnmarshalJSON(data []byte) error {
	type Alias DistributePayoutsEvent
	aux := &struct {
		Amount                        string `json:"amount"`
		Fee                           string `json:"fee"`
		BeneficiaryDistributionAmount string `json:"beneficiaryDistributionAmount"`
		*Alias
	}{
		Alias: (*Alias)(d),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var success bool
	if d.Amount, success = new(big.Int).SetString(aux.Amount, 10); !success {
		return fmt.Errorf("failed to parse amount: %s", aux.Amount)
	}
	if d.Fee, success = new(big.Int).SetString(aux.Fee, 10); !success {
		return fmt.Errorf("failed to parse fee: %s", aux.Fee)
	}
	if d.BeneficiaryDistributionAmount, success = new(big.Int).SetString(aux.BeneficiaryDistributionAmount, 10); !success {
		return fmt.Errorf("failed to parse beneficiaryDistributionAmount: %s", aux.BeneficiaryDistributionAmount)
	}
	return nil
}

// Custom UnmarshalJSON for SplitDistribution to handle big.Int
func (s *SplitDistribution) UnmarshalJSON(data []byte) error {
	type Alias SplitDistribution
	aux := &struct {
		Amount string `json:"amount"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var success bool
	if s.Amount, success = new(big.Int).SetString(aux.Amount, 10); !success {
		return fmt.Errorf("failed to parse amount: %s", aux.Amount)
	}
	return nil
}
