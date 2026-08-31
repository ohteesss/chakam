package queue

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

const (
	TypeProcessMessage = "message:process"
	TypeSendMessage    = "message:send"
)

type ProcessMessagePayload struct {
	From   string  `json:"from"`
	Name   *string `json:"name,omitempty"`
	Body   string  `json:"body"`
	Button *struct {
		Payload string `json:"payload"`
		Text    string `json:"text"`
	} `json:"button,omitempty"`
	Type      string  `json:"type"`
	MessageID *string `json:"message_id,omitempty"`
}

type SendMessagePayload struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

func NewProcessMessageTask(message ProcessMessagePayload) (*asynq.Task, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeProcessMessage, payload), nil
}

func NewSendMessageTask(to string, message string) (*asynq.Task, error) {
	payload, err := json.Marshal(SendMessagePayload{
		To:      to,
		Message: message,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendMessage, payload), nil
}
