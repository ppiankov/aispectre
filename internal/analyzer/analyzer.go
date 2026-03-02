package analyzer

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Thresholds for finding detection.
const (
	overkillMinRequests     = 50
	overkillMaxAvgOutput    = 100
	overkillMaxAvgInput     = 500
	cachingMinRequests      = 1000
	cachingMinHitRate       = 0.10
	costSpikeMultiplier     = 2.0
	costSpikeMinBaseline    = 1.0 // USD per day
	costSpikeRollingDays    = 7
	batchMinDailyRequests   = 100
	inefficiencyMinRequests = 50
	inefficiencyMaxRatio    = 10.0
	batchDiscount           = 0.50
	cacheDiscount           = 0.90 // cached input tokens cost ~10% of normal
	inefficiencyReduction   = 0.20
)

// Analyze examines normalized usage data and returns waste findings.
func Analyze(input Input, cfg Config) []Finding {
	var findings []Finding

	findings = append(findings, checkModelOverkill(input.TokenUsage)...)
	findings = append(findings, checkNoCaching(input.TokenUsage)...)
	findings = append(findings, checkKeyUnused(input.APIKeys, cfg)...)
	findings = append(findings, checkFinetunedIdle(input.Models, input.TokenUsage, cfg)...)
	findings = append(findings, checkCostSpike(input.TokenUsage, input.DailyCosts)...)
	findings = append(findings, checkBatchEligible(input.TokenUsage)...)
	findings = append(findings, checkTokenInefficiency(input.TokenUsage)...)
	findings = append(findings, checkDowngradeAvailable(input.TokenUsage)...)

	// Filter by MinWaste, except security findings.
	filtered := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if isSecurityFinding(f.Kind) || f.MonthlyWaste >= cfg.MinWaste {
			filtered = append(filtered, f)
		}
	}

	// Sort: severity descending, then monthly waste descending.
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Severity != filtered[j].Severity {
			return filtered[i].Severity > filtered[j].Severity
		}
		return filtered[i].MonthlyWaste > filtered[j].MonthlyWaste
	})

	return filtered
}

func isSecurityFinding(kind FindingKind) bool {
	return kind == KindKeyUnused || kind == KindFinetunedIdle
}

func checkModelOverkill(usage []TokenUsage) []Finding {
	type modelStats struct {
		totalInput, totalOutput, totalRequests int
		platform                               string
	}
	byModel := make(map[string]*modelStats)
	for _, u := range usage {
		s, ok := byModel[u.Model]
		if !ok {
			s = &modelStats{platform: u.Platform}
			byModel[u.Model] = s
		}
		s.totalInput += u.InputTokens
		s.totalOutput += u.OutputTokens
		s.totalRequests += u.Requests
	}

	var findings []Finding
	for model, s := range byModel {
		if s.totalRequests < overkillMinRequests {
			continue
		}
		p, ok := LookupModel(model)
		if !ok || p.Tier != TierLarge {
			continue
		}
		avgInput := s.totalInput / s.totalRequests
		avgOutput := s.totalOutput / s.totalRequests
		if avgOutput >= overkillMaxAvgOutput && avgInput >= overkillMaxAvgInput {
			continue
		}

		// Estimate savings: difference between current and tier-1 alternative.
		currentCost := TokenCost(model, s.totalInput, s.totalOutput)
		var savingsDesc string
		if p.DowngradeTo != "" {
			downgradeCost := TokenCost(p.DowngradeTo, s.totalInput, s.totalOutput)
			savings := currentCost - downgradeCost
			savingsDesc = fmt.Sprintf("Consider %s (est. savings $%.2f/mo)", p.DowngradeTo, savings*30/float64(max(1, windowDays(usage))))
		}

		findings = append(findings, Finding{
			Kind:     KindModelOverkill,
			Severity: SeverityMedium,
			Platform: s.platform,
			Model:    model,
			Title:    fmt.Sprintf("Expensive model %s used for small tasks", model),
			Description: fmt.Sprintf(
				"Avg input %d tokens, avg output %d tokens across %d requests. %s",
				avgInput, avgOutput, s.totalRequests, savingsDesc,
			),
			MonthlyWaste: MonthlyProjection(currentCost, windowDays(usage)) * 0.5, // conservative: 50% could be downgraded
			Evidence: map[string]any{
				"avg_input_tokens":  avgInput,
				"avg_output_tokens": avgOutput,
				"total_requests":    s.totalRequests,
				"tier":              p.Tier,
			},
		})
	}
	return findings
}

func checkNoCaching(usage []TokenUsage) []Finding {
	type cacheStats struct {
		totalInput, totalCached, totalRequests int
		platform                               string
	}
	type key struct{ platform, model string }
	byKey := make(map[key]*cacheStats)
	for _, u := range usage {
		k := key{u.Platform, u.Model}
		s, ok := byKey[k]
		if !ok {
			s = &cacheStats{platform: u.Platform}
			byKey[k] = s
		}
		s.totalInput += u.InputTokens
		s.totalCached += u.CachedTokens
		s.totalRequests += u.Requests
	}

	var findings []Finding
	for k, s := range byKey {
		if s.totalRequests < cachingMinRequests {
			continue
		}
		p, ok := LookupModel(k.model)
		if !ok || !p.SupportsCache {
			continue
		}
		if s.totalInput == 0 {
			continue
		}
		hitRate := float64(s.totalCached) / float64(s.totalInput)
		if hitRate >= cachingMinHitRate {
			continue
		}

		// Savings: if 50% of input could be cached, what would we save?
		potentialCached := float64(s.totalInput) * 0.5
		savingsPerToken := p.InputPerMTok * cacheDiscount / 1_000_000
		savings := potentialCached * savingsPerToken

		findings = append(findings, Finding{
			Kind:     KindNoCaching,
			Severity: SeverityLow,
			Platform: s.platform,
			Model:    k.model,
			Title:    fmt.Sprintf("Low cache utilization for %s", k.model),
			Description: fmt.Sprintf(
				"Cache hit rate %.1f%% across %d requests. Enabling prompt caching could reduce input costs.",
				hitRate*100, s.totalRequests,
			),
			MonthlyWaste: MonthlyProjection(savings, windowDays(usage)),
			Evidence: map[string]any{
				"cache_hit_rate": hitRate,
				"total_requests": s.totalRequests,
				"total_input":    s.totalInput,
				"cached_input":   s.totalCached,
				"supports_cache": true,
			},
		})
	}
	return findings
}

func checkKeyUnused(keys []APIKeyInfo, cfg Config) []Finding {
	var findings []Finding
	cutoff := cfg.Now.AddDate(0, 0, -cfg.IdleDays)

	for _, k := range keys {
		idle := false
		var desc string
		if k.LastUsedAt == nil {
			idle = true
			desc = fmt.Sprintf("API key %q has never been used", k.Name)
		} else if k.LastUsedAt.Before(cutoff) {
			idle = true
			days := int(cfg.Now.Sub(*k.LastUsedAt).Hours() / 24)
			desc = fmt.Sprintf("API key %q last used %d days ago", k.Name, days)
		}
		if !idle {
			continue
		}
		findings = append(findings, Finding{
			Kind:         KindKeyUnused,
			Severity:     SeverityHigh,
			Platform:     k.Platform,
			Title:        fmt.Sprintf("Unused API key: %s", k.Name),
			Description:  desc,
			MonthlyWaste: 0,
			Evidence: map[string]any{
				"key_id":       k.ID,
				"key_name":     k.Name,
				"created_at":   k.CreatedAt,
				"last_used_at": k.LastUsedAt,
			},
		})
	}
	return findings
}

func checkFinetunedIdle(models []ModelInfo, usage []TokenUsage, cfg Config) []Finding {
	// Build set of models that have any usage.
	usedModels := make(map[string]bool)
	for _, u := range usage {
		usedModels[u.Model] = true
	}

	var findings []Finding
	cutoff := cfg.Now.AddDate(0, 0, -cfg.IdleDays)

	for _, m := range models {
		if !isFineTuned(m) {
			continue
		}
		if usedModels[m.ID] {
			continue
		}
		if m.Created.After(cutoff) {
			continue // recently created, give it time
		}

		findings = append(findings, Finding{
			Kind:         KindFinetunedIdle,
			Severity:     SeverityMedium,
			Platform:     m.Platform,
			Model:        m.ID,
			Title:        fmt.Sprintf("Fine-tuned model %s has no inference", m.ID),
			Description:  "Fine-tuned model with zero requests in the observation window. Consider deleting if no longer needed.",
			MonthlyWaste: 0,
			Evidence: map[string]any{
				"model_id":   m.ID,
				"owned_by":   m.OwnedBy,
				"created_at": m.Created,
			},
		})
	}
	return findings
}

func isFineTuned(m ModelInfo) bool {
	if strings.HasPrefix(m.ID, "ft:") {
		return true
	}
	if strings.Contains(m.OwnedBy, "user-") || strings.Contains(m.OwnedBy, "org-") {
		return true
	}
	return false
}

func checkCostSpike(usage []TokenUsage, dailyCosts []DailyCost) []Finding {
	var days []DateAmount
	if len(dailyCosts) > 0 {
		days = DailySpendFromCosts(dailyCosts)
	} else {
		days = DailySpend(usage)
	}
	if len(days) < costSpikeRollingDays+1 {
		return nil
	}

	var findings []Finding
	for i := costSpikeRollingDays; i < len(days); i++ {
		var sum float64
		for j := i - costSpikeRollingDays; j < i; j++ {
			sum += days[j].Amount
		}
		avg := sum / float64(costSpikeRollingDays)
		if avg < costSpikeMinBaseline {
			continue
		}
		if days[i].Amount > avg*costSpikeMultiplier {
			spike := days[i].Amount - avg
			findings = append(findings, Finding{
				Kind:     KindCostSpike,
				Severity: SeverityHigh,
				Title:    fmt.Sprintf("Cost spike on %s", days[i].Date.Format("2006-01-02")),
				Description: fmt.Sprintf(
					"Daily spend $%.2f vs 7-day avg $%.2f (%.0f%% above average)",
					days[i].Amount, avg, (days[i].Amount/avg-1)*100,
				),
				MonthlyWaste: spike * 30, // if the spike persists
				Evidence: map[string]any{
					"date":      days[i].Date,
					"amount":    days[i].Amount,
					"avg_7d":    avg,
					"spike_pct": (days[i].Amount/avg - 1) * 100,
				},
			})
		}
	}
	return findings
}

func checkBatchEligible(usage []TokenUsage) []Finding {
	type modelDay struct {
		model string
		date  time.Time
	}
	dailyRequests := make(map[modelDay]int)
	modelPlatform := make(map[string]string)
	for _, u := range usage {
		day := u.StartTime.Truncate(24 * time.Hour)
		k := modelDay{u.Model, day}
		dailyRequests[k] += u.Requests
		modelPlatform[u.Model] = u.Platform
	}

	// Aggregate per model: avg daily requests.
	type modelAgg struct {
		totalRequests int
		days          int
	}
	byModel := make(map[string]*modelAgg)
	for k, reqs := range dailyRequests {
		a, ok := byModel[k.model]
		if !ok {
			a = &modelAgg{}
			byModel[k.model] = a
		}
		a.totalRequests += reqs
		a.days++
	}

	var findings []Finding
	for model, a := range byModel {
		p, ok := LookupModel(model)
		if !ok || !p.SupportsBatch {
			continue
		}
		avgDaily := a.totalRequests / a.days
		if avgDaily < batchMinDailyRequests {
			continue
		}

		// Savings: batch discount on total spend.
		ms := modelSpendForModel(usage, model)
		savings := ms.Cost * batchDiscount

		findings = append(findings, Finding{
			Kind:     KindBatchEligible,
			Severity: SeverityLow,
			Platform: modelPlatform[model],
			Model:    model,
			Title:    fmt.Sprintf("High-volume model %s eligible for batch API", model),
			Description: fmt.Sprintf(
				"Avg %d requests/day. Batch API offers ~50%% discount for non-latency-sensitive workloads.",
				avgDaily,
			),
			MonthlyWaste: MonthlyProjection(savings, windowDays(usage)),
			Evidence: map[string]any{
				"avg_daily_requests": avgDaily,
				"total_requests":     a.totalRequests,
				"supports_batch":     true,
			},
		})
	}
	return findings
}

func checkTokenInefficiency(usage []TokenUsage) []Finding {
	type modelStats struct {
		totalInput, totalOutput, totalRequests int
		platform                               string
	}
	byModel := make(map[string]*modelStats)
	for _, u := range usage {
		s, ok := byModel[u.Model]
		if !ok {
			s = &modelStats{platform: u.Platform}
			byModel[u.Model] = s
		}
		s.totalInput += u.InputTokens
		s.totalOutput += u.OutputTokens
		s.totalRequests += u.Requests
	}

	var findings []Finding
	for model, s := range byModel {
		if s.totalRequests < inefficiencyMinRequests || s.totalInput == 0 {
			continue
		}
		ratio := float64(s.totalOutput) / float64(s.totalInput)
		if ratio <= inefficiencyMaxRatio {
			continue
		}

		cost := TokenCost(model, s.totalInput, s.totalOutput)
		savings := cost * inefficiencyReduction

		findings = append(findings, Finding{
			Kind:     KindTokenInefficiency,
			Severity: SeverityLow,
			Platform: s.platform,
			Model:    model,
			Title:    fmt.Sprintf("High output/input ratio for %s", model),
			Description: fmt.Sprintf(
				"Output/input ratio %.1fx across %d requests. Consider constraining max_tokens or improving prompts.",
				ratio, s.totalRequests,
			),
			MonthlyWaste: MonthlyProjection(savings, windowDays(usage)),
			Evidence: map[string]any{
				"output_input_ratio": ratio,
				"total_input":        s.totalInput,
				"total_output":       s.totalOutput,
				"total_requests":     s.totalRequests,
			},
		})
	}
	return findings
}

func checkDowngradeAvailable(usage []TokenUsage) []Finding {
	type modelStats struct {
		totalInput, totalOutput int
		platform                string
	}
	byModel := make(map[string]*modelStats)
	for _, u := range usage {
		s, ok := byModel[u.Model]
		if !ok {
			s = &modelStats{platform: u.Platform}
			byModel[u.Model] = s
		}
		s.totalInput += u.InputTokens
		s.totalOutput += u.OutputTokens
	}

	var findings []Finding
	for model, s := range byModel {
		p, ok := LookupModel(model)
		if !ok || p.DowngradeTo == "" {
			continue
		}

		currentCost := TokenCost(model, s.totalInput, s.totalOutput)
		downgradeCost := TokenCost(p.DowngradeTo, s.totalInput, s.totalOutput)
		savings := currentCost - downgradeCost
		if savings <= 0 {
			continue
		}

		findings = append(findings, Finding{
			Kind:     KindDowngradeAvailable,
			Severity: SeverityMedium,
			Platform: s.platform,
			Model:    model,
			Title:    fmt.Sprintf("Cheaper alternative available for %s", model),
			Description: fmt.Sprintf(
				"Consider %s (est. savings $%.2f in observation window)",
				p.DowngradeTo, savings,
			),
			MonthlyWaste: MonthlyProjection(savings, windowDays(usage)),
			Evidence: map[string]any{
				"current_model":  model,
				"downgrade_to":   p.DowngradeTo,
				"current_cost":   currentCost,
				"downgrade_cost": downgradeCost,
				"savings":        savings,
			},
		})
	}
	return findings
}

// windowDays estimates the observation window in days from usage data.
func windowDays(usage []TokenUsage) int {
	if len(usage) == 0 {
		return 1
	}
	var earliest, latest time.Time
	for i, u := range usage {
		if i == 0 || u.StartTime.Before(earliest) {
			earliest = u.StartTime
		}
		if i == 0 || u.EndTime.After(latest) {
			latest = u.EndTime
		}
	}
	days := int(latest.Sub(earliest).Hours()/24) + 1
	if days < 1 {
		return 1
	}
	return days
}

// modelSpendForModel calculates total spend for a single model from usage data.
func modelSpendForModel(usage []TokenUsage, model string) ModelAmount {
	var ma ModelAmount
	ma.Model = model
	for _, u := range usage {
		if u.Model != model {
			continue
		}
		ma.InputTokens += u.InputTokens
		ma.OutputTokens += u.OutputTokens
		ma.Requests += u.Requests
	}
	ma.Cost = TokenCost(model, ma.InputTokens, ma.OutputTokens)
	return ma
}
