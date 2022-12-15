module github.com/nikogura/dbt

require (
	github.com/abbot/go-http-auth v0.4.0
	github.com/aws/aws-sdk-go v1.44.159
	github.com/fatih/color v1.10.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/johannesboyne/gofakes3 v0.0.0-20200218152459-de0855a40bc1
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/nikogura/gomason v0.0.0-20221214033104-316a595994bb
	github.com/orion-labs/jwt-ssh-agent-go v0.0.0-20200108200620-50a51684897c
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/rivo/uniseg v0.4.3 // indirect
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.6.1
	github.com/stretchr/testify v1.8.1
	golang.org/x/crypto v0.4.0
	golang.org/x/net v0.4.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.25
)

replace github.com/nikogura/gomason => /home/vayde/project/github.com/nikogura/gomason

go 1.12
