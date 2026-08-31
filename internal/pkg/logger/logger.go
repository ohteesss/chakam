package logger

import "go.uber.org/zap"

var Logger *zap.Logger

func InitLogger() {
	Logger, _ = zap.NewProduction()
}

func Info(message string, fields ...zap.Field) {
	Logger.Info(message, fields...)
}

func Error(message string, fields ...zap.Field) {
	Logger.Error(message, fields...)
}

func Fatal(message string, fields ...zap.Field) {
	Logger.Fatal(message, fields...)
}

func New() *zap.Logger {
	if Logger == nil {
		InitLogger()
	}
	return Logger
}

// MaskPhone keeps logs useful for correlation without recording a complete
// personal identifier.
func MaskPhone(phone string) string {
	if len(phone) <= 4 {
		return "****"
	}
	return "***" + phone[len(phone)-4:]
}
