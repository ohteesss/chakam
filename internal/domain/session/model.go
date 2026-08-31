package session

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"chakam/internal/domain/user"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ConversationHistory []Message

// Value is called when WRITING to the DB: turn the map into JSON bytes.
func (c ConversationHistory) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

// Scan is called when READING from the DB: parse JSON bytes back into the map.
func (c *ConversationHistory) Scan(src any) error {
	if src == nil {
		*c = nil
		return nil
	}
	b, ok := src.([]byte)
	if !ok {
		return errors.New("ConversationHistory: expected []byte from database")
	}
	return json.Unmarshal(b, c)
}

const (
	SessionTTL = 60 * time.Minute
)

type Session struct {
	ID                  string              `gorm:"primaryKey"`
	UserID              string              `gorm:"not null"`
	User                *user.User          `gorm:"foreignKey:UserID;references:PublicID"`
	CreatedAt           time.Time           `gorm:"autoCreateTime"`
	UpdatedAt           time.Time           `gorm:"autoUpdateTime"`
	ExpiresAt           time.Time           `gorm:"not null"`
	ConversationHistory ConversationHistory `gorm:"type:jsonb;default:'[]'"`
}

func (s *Session) IsExpired() bool {
	return s.ExpiresAt.Before(time.Now())
}
