package healthdemo

import (
	"github.com/amkargar/gonex/app"
	"github.com/gin-gonic/gin"
)

type Module struct{}

func (Module) Register(r *app.Registrar) error {
	return r.Handle("GET", "/demo", func(c *gin.Context) { c.JSON(200, gin.H{"message": "Gonex"}) })
}
