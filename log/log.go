package log

import (
	"os"

	"go.uber.org/zap"
)

var l *zap.Logger
var S *zap.SugaredLogger

func init() {
	var err error
	if os.Getenv("DEPLOY_ENV") == "dev" || os.Getenv("deploy_env") == "dev" {
		l, err = zap.NewDevelopment()
	} else {
		l, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}
	S = l.Sugar()
}

func Close() {
	l.Sync()
}
