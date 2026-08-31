package user

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)


type JSONB map[string]any

// Value is called when WRITING to the DB: turn the map into JSON bytes.
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan is called when READING from the DB: parse JSON bytes back into the map.
func (j *JSONB) Scan(src any) error {
	if src == nil {
		*j = nil
		return nil
	}
	b, ok := src.([]byte)
	if !ok {
		return errors.New("JSONB: expected []byte from database")
	}
	return json.Unmarshal(b, j)
}

type User struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement" json:"-"`
	PublicID string `gorm:"type:varchar(36);uniqueIndex;not null" json:"id"` // UUID v4 = 36 chars

	PhoneNumber string  `gorm:"type:varchar(20);uniqueIndex;not null" json:"phone_number"`
	Name        *string `gorm:"type:varchar(255)" json:"name,omitempty"`

	IsBlocked     bool    `gorm:"default:false" json:"is_blocked"`
	BlockedReason *string `gorm:"type:text" json:"blocked_reason,omitempty"`

	Metadata JSONB `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
