package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/nubank/lola-ia-backend/internal"
)

type OpenAIProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewOpenAIProvider(model string) (*OpenAIProvider, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY vacío")
	}
	if model == "" {
		model = "gpt-4.1-mini"
	}
	return &OpenAIProvider{
		apiKey: key,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (p *OpenAIProvider) Model() string { return p.model }

func (p *OpenAIProvider) Reply(history []internal.Message, userInput string) (string, error) {
	/*
		Usamos la API de Responses:
		POST https://api.openai.com/v1/responses
		Body:
		{
		  "model": "...",
		  "input": [
		    {"role":"system","content":"You are Lola IA..."},
		    {"role":"assistant","content":"..."},
		    {"role":"user","content":"..."},
		    ...
		  ]
		}
	*/

	type item struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := struct {
		Model string `json:"model"`
		Input []item `json:"input"`
	}{
		Model: p.model,
		Input: make([]item, 0, len(history)+2),
	}

	// Prompt del sistema mínimo
	payload.Input = append(payload.Input, item{
		Role:    "system",
		Content: "Eres Lola IA, un asistente breve y claro.",
	})

	for _, m := range history {
		payload.Input = append(payload.Input, item{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	// Último input del usuario
	payload.Input = append(payload.Input, item{
		Role:    "user",
		Content: userInput,
	})

	b, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		if e.Error.Message != "" {
			return "", errors.New(e.Error.Message)
		}
		return "", errors.New("openai error: " + resp.Status)
	}

	var out struct {
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}

	// Tomamos el primer bloque de texto
	if len(out.Output) > 0 && len(out.Output[0].Content) > 0 {
		return out.Output[0].Content[0].Text, nil
	}
	return "", errors.New("respuesta vacía de OpenAI")
}
