package provider

import "github.com/nubank/lola-ia-backend/internal"

type ChatProvider interface {
	Model() string
	Reply(history []internal.Message, userInput string) (string, error)
}

// Fallback provider (mock) que responde sin API externa.
type MockProvider struct{}

func (m MockProvider) Model() string { return "mock-lola-ia" }

func (m MockProvider) Reply(history []internal.Message, userInput string) (string, error) {
	// Respuesta simple para desarrollo offline
	return "Entendido. (mock) Me pediste: \"" + userInput + "\"", nil
}
