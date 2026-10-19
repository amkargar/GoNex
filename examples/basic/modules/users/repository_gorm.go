package users

import (
	"context"
	"crypto/rand"
	"errors"
	apperrors "github.com/amkargar/gonex/errors"
	"github.com/amkargar/gonex/orm"
	"gorm.io/gorm"
)

type GORMRepository struct{ database *orm.Database }

func NewGORMRepository(database *orm.Database) *GORMRepository {
	return &GORMRepository{database: database}
}
func (r *GORMRepository) Create(ctx context.Context, name, email string) (User, error) {
	db, err := r.database.DB()
	if err != nil {
		return User{}, err
	}
	model := UserModel{ID: rand.Text(), Name: name, Email: email}
	if err = db.WithContext(ctx).Create(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return User{}, apperrors.New(apperrors.Conflict, "email already exists")
		}
		return User{}, apperrors.New(apperrors.Unavailable, "database unavailable")
	}
	return User{ID: model.ID, Name: model.Name, Email: model.Email}, nil
}
func (r *GORMRepository) Get(ctx context.Context, id string) (User, error) {
	db, err := r.database.DB()
	if err != nil {
		return User{}, err
	}
	var model UserModel
	err = db.WithContext(ctx).First(&model, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, apperrors.New(apperrors.NotFound, "user not found")
	}
	if err != nil {
		return User{}, apperrors.New(apperrors.Unavailable, "database unavailable")
	}
	return User{ID: model.ID, Name: model.Name, Email: model.Email}, nil
}
