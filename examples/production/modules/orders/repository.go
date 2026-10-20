package orders

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	apperrors "github.com/amkargar/gonex/errors"
	"github.com/amkargar/gonex/orm"
	"gorm.io/gorm"
)

type model struct {
	ID        string `gorm:"primaryKey"`
	Item      string
	Quantity  int
	Status    string
	CreatedAt time.Time
}

func (model) TableName() string { return "orders" }

type Repository struct{ Database *orm.Database }

func (r Repository) db(ctx context.Context) (*gorm.DB, error) {
	db, err := r.Database.DB()
	if err != nil {
		return nil, apperrors.New(apperrors.Unavailable, "database unavailable")
	}
	return db.WithContext(ctx), nil
}
func domain(m model) Order {
	return Order{ID: m.ID, Item: m.Item, Quantity: m.Quantity, Status: m.Status, CreatedAt: m.CreatedAt}
}
func failure(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperrors.New(apperrors.NotFound, "order not found")
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return apperrors.New(apperrors.Conflict, "order conflict")
	}
	return apperrors.New(apperrors.Unavailable, "database unavailable")
}
func (r Repository) Create(ctx context.Context, item string, quantity int) (Order, error) {
	db, err := r.db(ctx)
	if err != nil {
		return Order{}, err
	}
	m := model{ID: rand.Text(), Item: item, Quantity: quantity, Status: "pending", CreatedAt: time.Now().UTC()}
	if err = db.Create(&m).Error; err != nil {
		return Order{}, failure(err)
	}
	return domain(m), nil
}
func (r Repository) Get(ctx context.Context, id string) (Order, error) {
	db, err := r.db(ctx)
	if err != nil {
		return Order{}, err
	}
	var m model
	if err = db.First(&m, "id = ?", id).Error; err != nil {
		return Order{}, failure(err)
	}
	return domain(m), nil
}
func (r Repository) List(ctx context.Context, f Filter) ([]Order, error) {
	db, err := r.db(ctx)
	if err != nil {
		return nil, err
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	var rows []model
	if err = db.Order("created_at DESC, id DESC").Limit(f.Limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return nil, failure(err)
	}
	result := make([]Order, 0, len(rows))
	for _, m := range rows {
		result = append(result, domain(m))
	}
	return result, nil
}
func (r Repository) Update(ctx context.Context, id, item string, quantity int, status string) (Order, error) {
	db, err := r.db(ctx)
	if err != nil {
		return Order{}, err
	}
	result := db.Model(&model{}).Where("id = ?", id).Updates(map[string]any{"item": item, "quantity": quantity, "status": status})
	if result.Error != nil {
		return Order{}, failure(result.Error)
	}
	if result.RowsAffected == 0 {
		return Order{}, apperrors.New(apperrors.NotFound, "order not found")
	}
	return r.Get(ctx, id)
}
func (r Repository) Delete(ctx context.Context, id string) error {
	db, err := r.db(ctx)
	if err != nil {
		return err
	}
	result := db.Delete(&model{}, "id = ?", id)
	if result.Error != nil {
		return failure(result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.New(apperrors.NotFound, "order not found")
	}
	return nil
}
