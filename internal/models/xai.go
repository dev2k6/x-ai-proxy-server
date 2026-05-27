package models

type XAIRequest struct {
	Model           string       `json:"model"`
	Input           []XAIInput   `json:"input"`
	MaxOutputTokens int          `json:"max_output_tokens"`
	Temperature     float64      `json:"temperature"`
	TopP            float64      `json:"top_p"`
	Reasoning       XAIReasoning `json:"reasoning"`
	Store           bool         `json:"store"`
	Include         []string     `json:"include"`
	Tools           interface{}  `json:"tools,omitempty"`
	ToolChoice      interface{}  `json:"tool_choice,omitempty"`
	ResponseFormat  interface{}  `json:"response_format,omitempty"`
	Stream          bool         `json:"stream"`
}

type XAIInput struct {
	Role    string       `json:"role"`
	Content []XAIContent `json:"content"`
}

type XAIContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type XAIReasoning struct {
	Effort string `json:"effort"`
}

type XAITool struct {
	Type                     string `json:"type"`
	EnableImageUnderstanding bool   `json:"enable_image_understanding,omitempty"`
	EnableVideoUnderstanding bool   `json:"enable_video_understanding,omitempty"`
}

type XAIEvent struct {
	Type      string `json:"type"`
	Delta     string `json:"delta,omitempty"`
	Text      string `json:"text,omitempty"`
	Sequence  int    `json:"sequence_number,omitempty"`
	Response  *XAIResponse `json:"response,omitempty"`
}

type XAIResponse struct {
	ID        string `json:"id"`
	Object    string `json:"object"`
	CreatedAt int64  `json:"created_at"`
	Model     string `json:"model"`
	Status    string `json:"status"`
}