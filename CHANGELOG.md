# Changelog

All notable changes to aispectre will be documented in this file.

## [0.1.0] - 2026-03-21

### Added

- Cobra CLI with `scan`, `init`, and `version` commands
- OpenAI usage client (organization costs, completion usage, API keys, models)
- Anthropic usage client with pagination
- AWS Bedrock client (CloudWatch metrics)
- Azure OpenAI client (Azure Monitor metrics)
- Vertex AI client (Cloud Monitoring)
- Cohere stub client (no public usage API)
- Analyzer with 25-model pricing database across 3 tiers
- 8 finding types: MODEL_OVERKILL, NO_CACHING, KEY_UNUSED, FINETUNED_IDLE, COST_SPIKE, BATCH_ELIGIBLE, TOKEN_INEFFICIENCY, DOWNGRADE_AVAILABLE
- Text, JSON, and spectre/v1 output formats
- YAML config loader with environment variable auto-detection
- End-to-end scan wiring across all 6 platforms

## [Unreleased]
