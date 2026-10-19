package ordershttp

type Request struct {
	ID string `path:"id" validate:"required"`
}
type Response struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
