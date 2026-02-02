module github.com/neves/zen-claw

go 1.25

toolchain go1.25

// Zen SDK integration notes:
// Some components from ~/zen/zen-sdk could be integrated for:
// - Enhanced logging (github.com/neves/zen-sdk/pkg/logging)
// - Better configuration management (github.com/neves/zen-sdk/pkg/config)
// - Standardized error handling (github.com/neves/zen-sdk/pkg/errors)
// - Health checks (github.com/neves/zen-sdk/pkg/health)
// - Metrics collection (github.com/neves/zen-sdk/pkg/metrics)

require (
	github.com/sashabaranov/go-openai v1.41.2
	github.com/spf13/cobra v1.8.1
	github.com/tmc/langchaingo v0.1.14
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pkoukk/tiktoken-go v0.1.6 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)
