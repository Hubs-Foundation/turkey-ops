package internal

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func InitLogger() {
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "t"
	encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout("060102.03:04:05MST") //wanted to use time.Kitchen so much
	encoderCfg.CallerKey = "c"
	encoderCfg.FunctionKey = "f"
	encoderCfg.MessageKey = "m"
	// encoderCfg.FunctionKey = "f"
	logger = zap.New(zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stdout), atom), zap.AddCaller())

	defer logger.Sync()

	atom.SetLevel(zap.DebugLevel)
}
