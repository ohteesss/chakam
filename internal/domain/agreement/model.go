package agreement

import "time"

type Money struct {
	AmountMinor int64  `gorm:"column:amount_minor"`
	Currency    string `gorm:"column:currency;type:varchar(3)"`
}

type Agreement struct {
	ID             string    `gorm:"primaryKey"`
	InitiatorID    string    `gorm:"not null"`
	CounterpartyID string    `gorm:"not null"`
	AgreementType  string    `gorm:"not null"`
	Description    string    `gorm:"not null"`
	Amount         Money     `gorm:"embedded"`
	Status         string    `gorm:"not null"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}
