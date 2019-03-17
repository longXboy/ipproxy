package log

import (
	"go.uber.org/zap"
)

var l *zap.Logger
var S *zap.SugaredLogger

func Init() {
	var err error
	l, err = zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	S = l.Sugar()
}

func Close() {
	l.Sync()
}
