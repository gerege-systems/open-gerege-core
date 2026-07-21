// Government Template Platform V3.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package logger_test

import (
	"context"
	"testing"

	"template/pkg/logger"
)

func TestLoggerUsage(t *testing.T) {
	// Анхдагч logger-ийг эхлүүлэх
	config := logger.Config{
		Level:             logger.LevelDebug,
		EnableConsole:     true,
		ConsoleJSONFormat: true,
		EnableFile:        false,
		AppName:           "my-service",
	}

	err := logger.InitDefault(config, logger.InstanceZap)
	if err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	// Жишээ 1: Үндсэн лог бичих
	logger.Info("Application started")
	logger.Debugf("Debug message with value: %d", 42)

	// Жишээ 2: Бүтэцлэгдсэн лог бичих
	logger.Info("User action", logger.Fields{
		"user_id": "12345",
		"action":  "login",
		"ip":      "192.168.1.1",
	})

	// Жишээ 3: Context-ийг мэддэг лог бичих
	ctx := context.WithValue(context.Background(), logger.TraceIDKey, "trace-123-456")
	logger.InfoWithContext(ctx, "Processing request")
	logger.InfofWithContext(ctx, "Processing request for user: %s", "john")

	// Жишээ 4: Талбаруудтай гинжлэгдсэн (chained) лог бичих
	log := logger.WithFields(logger.Fields{
		"component": "auth",
		"module":    "login",
	})
	log.Info("Authentication started")
	log.Warn("Failed login attempt")

	// Жишээ 5: WithContext болон WithFields-г хослуулах
	logWithCtx := logger.WithContext(ctx).WithFields(logger.Fields{
		"component": "handler",
	})
	logWithCtx.Info("Request processed successfully")

	// Жишээ 6: Алдааны лог бичих
	logger.Error("Database connection failed", logger.Fields{
		"error":    "connection timeout",
		"host":     "localhost",
		"port":     5432,
		"attempts": 3,
	})

	// Жишээ 7: Тусгай (custom) logger instance үүсгэх
	customConfig := logger.Config{
		Level:             logger.LevelWarn,
		EnableConsole:     true,
		ConsoleJSONFormat: false,
		EnableFile:        true,
		FileLocation:      "app.log",
		FileJSONFormat:    true,
		AppName:           "custom-service",
	}

	customLogger, err := logger.NewLogger(customConfig, logger.InstanceZap)
	if err != nil {
		t.Fatalf("failed to create custom logger: %v", err)
	}

	customLogger.Warn("This is a warning from custom logger")
}

func TestServiceLogger(t *testing.T) {
	// Logger-ийг эхлүүлэх
	config := logger.Config{
		Level:             logger.LevelDebug,
		EnableConsole:     true,
		ConsoleJSONFormat: true,
		AppName:           "user-service",
	}

	err := logger.InitDefault(config, logger.InstanceZap)
	if err != nil {
		t.Fatalf("failed to initialize logger: %v", err)
	}

	// Сервисийн аргыг загварчлах
	ctx := context.WithValue(context.Background(), logger.TraceIDKey, "req-789")

	// Нийтлэг талбаруудтай сервис түвшний logger
	serviceLogger := logger.WithFields(logger.Fields{
		"service": "user-service",
		"version": "1.0.0",
	})

	// Сервис дотор лог бичих
	serviceLogger.InfoWithContext(ctx, "Fetching user profile", logger.Fields{
		"user_id": "user-123",
	})

	// Боловсруулалтыг загварчлах
	processLogger := serviceLogger.WithContext(ctx).WithFields(logger.Fields{
		"operation": "update_profile",
	})

	processLogger.Debug("Validating input data")
	processLogger.Info("Updating user profile")

	// Алдааг загварчлах
	processLogger.Error("Failed to update profile", logger.Fields{
		"error":  "validation failed",
		"field":  "email",
		"reason": "invalid format",
	})
}

func ExampleLogger() {
	// Logger-ийг эхлүүлэх
	config := logger.Config{
		Level:             logger.LevelInfo,
		EnableConsole:     true,
		ConsoleJSONFormat: true,
		AppName:           "example-app",
	}

	logger.InitDefault(config, logger.InstanceZap)

	// Үндсэн хэрэглээ
	logger.Info("Application started")

	// Context-той
	ctx := context.WithValue(context.Background(), logger.TraceIDKey, "trace-001")
	logger.InfoWithContext(ctx, "Request received")

	// Талбаруудтай
	logger.Info("User action", logger.Fields{
		"user_id": "123",
		"action":  "purchase",
		"amount":  99.99,
	})

	// Гинжилсэн хэлбэрээр ашиглах жишээ
	log := logger.WithContext(ctx).WithFields(logger.Fields{
		"component": "payment",
	})
	log.Info("Processing payment")
}
