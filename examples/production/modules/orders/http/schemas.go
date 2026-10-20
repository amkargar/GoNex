package ordershttp

import "time"

type Body struct {
	Item     string `json:"item" validate:"required,min=2,max=80"`
	Quantity int    `json:"quantity" validate:"required,min=1,max=1000"`
}
type CreateRequest struct {
	Body Body `body:"true"`
}
type GetRequest struct {
	ID string `path:"id" validate:"required,max=80"`
}
type UpdateBody struct {
	Item     string `json:"item" validate:"required,min=2,max=80"`
	Quantity int    `json:"quantity" validate:"required,min=1,max=1000"`
	Status   string `json:"status" validate:"required,max=20"`
}
type UpdateRequest struct {
	ID   string     `path:"id" validate:"required,max=80"`
	Body UpdateBody `body:"true"`
}
type ListRequest struct {
	Limit  *int    `query:"limit" validate:"omitempty,min=1,max=50"`
	Offset *int    `query:"offset" validate:"omitempty,min=0,max=10000"`
	Status *string `query:"status" validate:"omitempty,max=20"`
}
type Response struct {
	ID        string    `json:"id"`
	Item      string    `json:"item"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
type ListResponse struct {
	Items  []Response `json:"items"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}
