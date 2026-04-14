package user

type User struct {
    ID        uint64         `gorm:"primaryKey;autoIncrement" json:"-"`
    PublicID  string         `gorm:"type:varchar(21);uniqueIndex;not null" json:"id"`
    
    PhoneNumber string       `gorm:"type:varchar(20);uniqueIndex;not null" json:"phone_number"`
    Name        *string      `gorm:"type:varchar(255)" json:"name,omitempty"`
    
    IsBlocked      bool    `gorm:"default:false" json:"is_blocked"`
    BlockedReason  *string `gorm:"type:text" json:"blocked_reason,omitempty"`
    
    Metadata JSONB `gorm:"type:jsonb;default:'{}'" json:"metadata"`
    
    CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}
