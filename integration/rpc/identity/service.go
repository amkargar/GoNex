package identity

import (
	"context"
	apperrors "github.com/amkargar/gonex/errors"
)

type User struct{ID,Name,Email string}
type Service struct{}
func(Service)GetUser(ctx context.Context,id string)(User,error){
	if id=="slow"{<-ctx.Done();return User{},ctx.Err()}
	if err:=ctx.Err();err!=nil{return User{},err};if id!="1"{return User{},apperrors.New(apperrors.NotFound,"user not found")};return User{ID:"1",Name:"Ada",Email:"ada@example.com"},nil
}
