package tests

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	"github.com/StepOne-ai/vk_internship/internal/logger"

	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
	// Перехватываем вывод логов
	old := log.Writer()
	r, w, _ := os.Pipe()
	log.SetOutput(w)
	defer func() {
		log.SetOutput(old)
		w.Close()
	}()

	logger.SetupLogger(logger.DebugLevel)

	ctx := context.Background()
	logger.Info(ctx, "Test message", logger.Field{Key: "test_key", Value: "test_value"})

	w.Close()
	out, _ := io.ReadAll(r)

	assert.Contains(t, string(out), "[INFO] Test message test_key=test_value")
}
