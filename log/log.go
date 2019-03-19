package log

import (
	"go.uber.org/zap"
)

var l *zap.Logger
var S *zap.SugaredLogger

func init() {
	var err error
	l, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
	S = l.Sugar()
}

func Close() {
	l.Sync()
}
