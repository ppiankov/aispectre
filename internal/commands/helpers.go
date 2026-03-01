package commands

const sampleConfig = `# aispectre configuration
# See: https://github.com/ppiankov/aispectre

# Platforms to scan (at least one required for 'aispectre scan --all')
# Each platform needs an API token — set via config, env var, or --token flag.
#
# platforms:
#   openai:
#     # Token: set OPENAI_API_KEY env var, or uncomment below
#     # token: sk-...
#     enabled: true
#
#   anthropic:
#     # Token: set ANTHROPIC_API_KEY env var, or uncomment below
#     # token: sk-ant-...
#     enabled: true
#
#   bedrock:
#     # Uses AWS credentials (profile or env vars)
#     # profile: default
#     # region: us-east-1
#     enabled: false
#
#   azureopenai:
#     # Token: set AZURE_OPENAI_API_KEY env var
#     # endpoint: https://your-resource.openai.azure.com
#     enabled: false
#
#   vertexai:
#     # Uses Google Cloud credentials (ADC or service account)
#     # project: my-gcp-project
#     # region: us-central1
#     enabled: false
#
#   cohere:
#     # Token: set COHERE_API_KEY env var, or uncomment below
#     # token: ...
#     enabled: false

# Scan settings
# window: 30           # Lookback window for usage data (days)
# idle_days: 7         # Days of inactivity before flagging as idle
# min_waste: 1.0       # Minimum monthly waste to report ($)

# Output format: text, json, sarif, spectrehub
# format: text
`
