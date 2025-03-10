package util

import (
	"log"
	"os"
	"strconv"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func initLogger() (*zap.Logger, error) {
	logLevelEnv := os.Getenv("LOG_LEVEL")
	logLevelInt, err := strconv.Atoi(logLevelEnv)
	if err != nil {
		logLevelInt = int(zapcore.InfoLevel)
	}

	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(zapcore.Level(logLevelInt))
	zapCfg.EncoderConfig.CallerKey = "ln"
	zapCfg.EncoderConfig.FunctionKey = "fn"
	zapCfg.EncoderConfig.LevelKey = "lv"
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := zapCfg.Build()
	if err != nil {
		return nil, err
	}
	return logger, nil
}

func NewLogger() (*zap.Logger, func()) {
	logger, err := initLogger()
	if err != nil {
		log.Fatalf("fail to init logger, error: %v", err)
	}

	undo := zap.ReplaceGlobals(logger)

	return logger, func() {
		undo()
		_ = logger.Sync()
	}
}
