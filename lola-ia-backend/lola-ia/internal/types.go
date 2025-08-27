package internal

import "time"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatHistory struct {
	Messages []Message `json:"messages"`
}

type SendMessageRequest struct {
	Content string `json:"content"`
}

type SendMessageResponse struct {
	Reply Message `json:"reply"`
	Model string  `json:"model"`
}

// --- Knowledge base (CSV files) ---
type KnowledgeFile struct {
	Name string `json:"name"`
	Size int    `json:"size"`
	Text string `json:"text"`
}

type UploadFilesRequest struct {
	Files []KnowledgeFile `json:"files"`
}

type UploadFilesResponse struct {
	Count int `json:"count"`
	Total int `json:"total"`
}
