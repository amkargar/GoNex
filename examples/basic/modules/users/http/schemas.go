package usershttp

type CreateUser struct {
	Name  string `json:"name" validate:"required,min=1,max=100"`
	Email string `json:"email" validate:"required,email"`
}
type CreateRequest struct {
	Body CreateUser `body:"true"`
}
type GetRequest struct {
	ID string `path:"id" validate:"required"`
}
type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
