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
	PayEvents               []PayEvent               `json:"payEvents"`
}

type DistributePayoutsEvent struct {
	DistributedAmount  *big.Int            `json:"distributedAmount"`
	SplitDistributions []SplitDistribution `json:"splitDistributions"`
	TxHash             string              `json:"txHash"`
}

type PayEvent struct {
	ProjectID int      `json:"projectId"`
	Amount    *big.Int `json:"amount"`
}

type SplitDistribution struct {
	SplitProjectID int   `json:"splitProjectId"`
	Percent        int64 `json:"percent"`
}

// Custom UnmarshalJSON for DistributePayoutsEvent to handle big.Int
func (d *DistributePayoutsEvent) UnmarshalJSON(data []byte) error {
	type Alias DistributePayoutsEvent
	aux := &struct {
		DistributedAmount string `json:"distributedAmount"`
		*Alias
	}{
		Alias: (*Alias)(d),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var success bool
	if d.DistributedAmount, success = new(big.Int).SetString(aux.DistributedAmount, 10); !success {
		return fmt.Errorf("failed to parse amount: %s", aux.DistributedAmount)
	}
	return nil
}

func (p *PayEvent) UnmarshalJSON(data []byte) error {
	type Alias PayEvent
	aux := &struct {
		Amount string `json:"amount"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var success bool
	if p.Amount, success = new(big.Int).SetString(aux.Amount, 10); !success {
		return fmt.Errorf("failed to parse amount: %s", aux.Amount)
	}
	return nil
}
