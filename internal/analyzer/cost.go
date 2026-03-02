package analyzer

import (
	"sort"
	"time"
)

// DateAmount pairs a calendar date with a USD amount.
type DateAmount struct {
	Date   time.Time
	Amount float64
}

// ModelAmount aggregates token usage and estimated cost for a single model.
type ModelAmount struct {
	Model        string
	InputTokens  int
	OutputTokens int
	Requests     int
	Cost         float64
}

// DailySpend groups TokenUsage by date and estimates daily cost using the model DB.
func DailySpend(usage []TokenUsage) []DateAmount {
	byDate := make(map[time.Time]float64)
	for _, u := range usage {
		day := u.StartTime.Truncate(24 * time.Hour)
		byDate[day] += TokenCost(u.Model, u.InputTokens, u.OutputTokens)
	}
	out := make([]DateAmount, 0, len(byDate))
	for d, a := range byDate {
		out = append(out, DateAmount{Date: d, Amount: a})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

// DailySpendFromCosts converts DailyCost entries (from a costs API) into DateAmounts.
func DailySpendFromCosts(costs []DailyCost) []DateAmount {
	byDate := make(map[time.Time]float64)
	for _, c := range costs {
		day := c.Date.Truncate(24 * time.Hour)
		byDate[day] += c.Amount
	}
	out := make([]DateAmount, 0, len(byDate))
	for d, a := range byDate {
		out = append(out, DateAmount{Date: d, Amount: a})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

// ModelSpend groups TokenUsage by model and calculates per-model totals.
func ModelSpend(usage []TokenUsage) []ModelAmount {
	type acc struct {
		input, output, requests int
	}
	byModel := make(map[string]*acc)
	for _, u := range usage {
		a, ok := byModel[u.Model]
		if !ok {
			a = &acc{}
			byModel[u.Model] = a
		}
		a.input += u.InputTokens
		a.output += u.OutputTokens
		a.requests += u.Requests
	}
	out := make([]ModelAmount, 0, len(byModel))
	for model, a := range byModel {
		out = append(out, ModelAmount{
			Model:        model,
			InputTokens:  a.input,
			OutputTokens: a.output,
			Requests:     a.requests,
			Cost:         TokenCost(model, a.input, a.output),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cost > out[j].Cost })
	return out
}

// MonthlyProjection extrapolates a total cost from an observed window to 30 days.
func MonthlyProjection(total float64, windowDays int) float64 {
	if windowDays <= 0 {
		return 0
	}
	return total / float64(windowDays) * 30
}
