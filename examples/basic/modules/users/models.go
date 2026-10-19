package users

// UserModel is private to persistence adapters; the service uses User values.
type UserModel struct {
	ID    string `gorm:"primaryKey"`
	Name  string
	Email string `gorm:"uniqueIndex"`
}

func (UserModel) TableName() string { return "users" }
