//go:build exclude || ignore
// +build exclude ignore

package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type HelloWorld struct {
	Client *http.Client `json:"client"`
}

func (p *HelloWorld) Handler() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "hello from {{.ProjectName}}",
			"short":   "{{.ServerShortDesc}}",
			"desc":    "{{.ServerLongDesc}}",
		})
	}
}
