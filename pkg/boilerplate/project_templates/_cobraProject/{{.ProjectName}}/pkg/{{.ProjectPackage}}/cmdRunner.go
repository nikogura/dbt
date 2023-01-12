package {{.ProjectPackage}}

import (
	"fmt"
	"github.com/{{.ProjectName}}/pkg/config"
)

func Run(cfg config.Config) error {
	fmt.Printf("From {{.ProjectName}} Hello World!\n")
	fmt.Printf("Config: %+v\n", cfg)
	return nil
}
