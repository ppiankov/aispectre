# aispectre

[![CI](https://github.com/ppiankov/aispectre/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/aispectre/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ppiankov/aispectre)](https://goreportcard.com/report/github.com/ppiankov/aispectre)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

The bill auditor your AI spend doesn't want you to have.

aispectre scans your LLM platform usage and finds the waste hiding in plain sight — overprovisioned models, idle API keys, missed caching opportunities, cost spikes nobody noticed. It reads token counts and billing data, never prompt content, and tells you exactly where the money is going.

## What aispectre is

- A CLI that audits AI/LLM spend across 6 platforms from a single command
- A static analyzer with a 25-model pricing database that spots overkill, inefficiency, and drift
- A privacy-first tool that reads only token counts, model names, costs, and timestamps
- A reporter that outputs human-readable text, JSON, or spectre/v1 envelopes for automation
- A mirror that shows you what your bill already knows — but organized

## What aispectre is NOT

- Not a billing dashboard — it does not track or store historical spend
- Not a prompt inspector — it never reads, logs, or transmits prompt content
- Not a cost optimizer that changes your code or model routing
- Not a real-time monitor — it runs on-demand against a lookback window
- Not a replacement for platform billing consoles — it is a second opinion

## Philosophy

**Privacy is structural, not policy.** aispectre cannot access prompt content because it never asks for it. The API endpoints it calls return token counts and costs — that is the ceiling, not a self-imposed limit.

**Mirrors, not oracles.** The tool presents evidence and math. It does not decide what you should do. A finding says "you spent $X on GPT-4 for tasks that averaged 50 output tokens" — the decision is yours.

**Statistical signals only.** Every finding is derived from usage patterns: token ratios, request volumes, cost trends, idle periods. No ML, no heuristics that require tuning, no probabilistic guesses.

## Quick Start

```sh
brew install ppiankov/tap/aispectre
export OPENAI_API_KEY=sk-...
aispectre scan --platform openai
```

Or build from source:

```sh
git clone https://github.com/ppiankov/aispectre.git
cd aispectre
make build
./bin/aispectre scan --platform openai
```

## Usage

### Scan a single platform

```sh
aispectre scan --platform openai
aispectre scan --platform anthropic --format json
aispectre scan --platform bedrock --window 14
```

### Scan all configured platforms

```sh
aispectre scan --all
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--platform` | | Platform to scan (openai, anthropic, bedrock, azureopenai, vertexai, cohere, groq) |
| `--all` | `false` | Scan all configured platforms |
| `--window` | `30` | Lookback window in days |
| `--idle-days` | `7` | Days of inactivity before flagging as idle |
| `--min-waste` | `1.0` | Minimum monthly waste to report ($) |
| `--format` | `text` | Output format: text, json, spectrehub |
| `--token` | | API token (overrides config and env) |

### Output formats

- **text** — human-readable report with severity sections and savings table
- **json** — structured JSON with findings array
- **spectrehub** — spectre/v1 envelope with `$schema`, target, summary, and findings

### Config file

Create `.aispectre.yaml` in your project or home directory:

```yaml
window: 30
idle_days: 7
min_waste: 1.0
format: text

platforms:
  openai:
    enabled: true
  anthropic:
    enabled: true
  bedrock:
    enabled: true
    region: us-east-1
```

Or generate a starter config:

```sh
aispectre init
```

Platform detection also works automatically from environment variables (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `AWS_PROFILE`, `AZURE_SUBSCRIPTION_ID`, `GOOGLE_CLOUD_PROJECT`, `COHERE_API_KEY`).

## Supported Platforms

| Platform | Auth | Data Source |
|----------|------|-------------|
| OpenAI | `OPENAI_API_KEY` (admin key) | Organization Usage + Costs API |
| Anthropic | `ANTHROPIC_API_KEY` (admin key) | Usage Report API |
| AWS Bedrock | `AWS_PROFILE` / IAM | CloudWatch Metrics |
| Azure OpenAI | `AZURE_SUBSCRIPTION_ID` | Azure Monitor Metrics |
| Vertex AI | `GOOGLE_CLOUD_PROJECT` | Cloud Monitoring |
| Cohere | `COHERE_API_KEY` | Stub (no usage API available) |
| Groq | `GROQ_API_KEY` | Stub (no usage API available) |

## Finding Types

| Finding | Severity | What it detects |
|---------|----------|-----------------|
| `MODEL_OVERKILL` | High | Expensive model used where a cheaper tier suffices |
| `NO_CACHING` | Medium | High request volume with no cached token usage |
| `KEY_UNUSED` | Medium | API key with zero usage in N days |
| `FINETUNED_IDLE` | Medium | Fine-tuned model with zero inference in N days |
| `COST_SPIKE` | High | Daily spend exceeds 2x the rolling 7-day average |
| `BATCH_ELIGIBLE` | Low | High-volume calls eligible for batch API |
| `TOKEN_INEFFICIENCY` | Low | Output-to-input ratio suggests inefficient prompting |
| `DOWNGRADE_AVAILABLE` | Info | Cheaper model available in the same tier |

## Architecture

```
aispectre scan --platform openai
       │
       ▼
┌─────────────┐     ┌──────────┐     ┌──────────────┐     ┌──────────┐     ┌──────────┐
│   CLI       │────▶│  Config  │────▶│  Platform     │────▶│ Analyzer │────▶│ Reporter │
│  (Cobra)    │     │  Loader  │     │  Clients (6)  │     │          │     │          │
└─────────────┘     └──────────┘     └──────────────┘     └──────────┘     └──────────┘
                    .aispectre.yaml   openai, anthropic    25-model DB      text, json,
                    + env detection   bedrock, azure,      8 finding        spectrehub
                                      vertexai, cohere     types
```

### Admin keys required (OpenAI + Anthropic)

Both OpenAI and Anthropic usage endpoints are organization-level admin APIs. Regular API keys will return 401/403.

| Platform | Regular key prefix | Admin key prefix | Where to create |
|----------|-------------------|------------------|-----------------|
| OpenAI | `sk-proj-` | `sk-admin-` | Settings > Organization > Admin keys |
| Anthropic | `sk-ant-api03-` | `sk-ant-admin-` | Console > Manage > API keys > Admin keys tab |

Anthropic admin keys are only available on **Team or Enterprise** plans. Individual orgs do not have the Admin keys tab — the usage API is not accessible for individual accounts.

### Azure OpenAI: CLI auth required

Azure OpenAI uses `DefaultAzureCredential` which tries multiple auth methods in order. The simplest for local use:

```sh
brew install azure-cli
az login
```

Set these env vars (or use `.aispectre.yaml`):

```sh
export AZURE_SUBSCRIPTION_ID=your-subscription-id
export AZURE_RESOURCE_GROUP=your-resource-group
export AZURE_OPENAI_ACCOUNT=your-account-name   # first part of your endpoint URL
```

### Vertex AI: GCP auth required

Vertex AI uses Application Default Credentials:

```sh
brew install google-cloud-sdk
gcloud auth application-default login
export GOOGLE_CLOUD_PROJECT=your-project-id
```

## Known Limitations

- **Cohere** and **Groq** have no public usage API — clients are stubs that return gracefully
- **SARIF** output format is stubbed (returns an error) — planned for a future release
- **No prompt content** — by design. aispectre cannot tell you what your prompts say, only how many tokens they used
- **No historical tracking** — each scan is a point-in-time snapshot against the lookback window
- **Rate limits** — platform API rate limits apply; aispectre does not retry on 429s yet

## Roadmap

- [ ] SARIF output for CI/CD integration
- [ ] GitHub Actions integration (scan on PR, comment with findings)
- [ ] Historical trend tracking (local SQLite)
- [ ] Additional platforms (Mistral, Together, Fireworks)
- [ ] Webhook / Slack notifications for cost spikes
- [ ] Interactive TUI mode

## License

[MIT](LICENSE)
