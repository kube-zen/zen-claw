package providers

// ProviderConfig holds configuration for AI providers
type ProviderConfig struct {
	APIKey  string
	Model   string
	BaseURL string // Optional custom base URL
}