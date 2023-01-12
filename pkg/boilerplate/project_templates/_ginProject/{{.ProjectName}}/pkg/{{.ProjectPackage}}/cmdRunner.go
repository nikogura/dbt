// +build exclude {{.ProjectName}}

package {{.ProjectPackage}}

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/{{.ProjectName}}/pkg/config"
	"github.com/{{.ProjectName}}/pkg/handlers"
	"net/http"
)

func Run(cfg config.Config) error {
	eng := gin.Default()

	root := eng.Group("/api")

	hw := handlers.HelloWorld{
		Client:           http.DefaultClient,
	}

	root.GET("/helloworld", hw.Handler())
	return eng.Run(fmt.Sprintf(":%d", cfg.Port))
}
