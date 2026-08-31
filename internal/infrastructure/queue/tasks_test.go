package queue

import (
	"encoding/json"
	"testing"
)

func TestNewProcessMessageTaskPreservesButton(t *testing.T) {
	messageID := "wamid.123"
	payload := ProcessMessagePayload{
		From:      "234801",
		Type:      "button",
		MessageID: &messageID,
		Button: &struct {
			Payload string `json:"payload"`
			Text    string `json:"text"`
		}{Payload: "ACCEPT:req-1", Text: "Accept"},
	}
	task, err := NewProcessMessageTask(payload)
	if err != nil {
		t.Fatalf("NewProcessMessageTask() error = %v", err)
	}

	var decoded ProcessMessagePayload
	if err := json.Unmarshal(task.Payload(), &decoded); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if decoded.Button == nil || decoded.Button.Payload != payload.Button.Payload {
		t.Fatalf("button = %#v, want %#v", decoded.Button, payload.Button)
	}
}
