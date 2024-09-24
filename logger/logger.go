package logger

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

type RateLimitedLogger struct {
	logger      *log.Logger
	fileLogger  *log.Logger
	lastLogTime map[string]time.Time
	logInterval time.Duration
	mu          sync.Mutex
}

func NewRateLimitedLogger(logDir string) (*RateLimitedLogger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(filepath.Join(logDir, "gitspace.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &RateLimitedLogger{
		logger:      log.New(os.Stderr),
		fileLogger:  log.New(logFile),
		lastLogTime: make(map[string]time.Time),
		logInterval: time.Second * 5, // Log the same message at most once every 5 seconds
	}, nil
}

func (l *RateLimitedLogger) Log(level log.Level, message string, keyvals ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if lastLog, exists := l.lastLogTime[message]; !exists || now.Sub(lastLog) >= l.logInterval {
		l.logger.Log(level, message, keyvals...)
		l.fileLogger.Log(level, message, keyvals...)
		l.lastLogTime[message] = now
	}
}

func (l *RateLimitedLogger) Info(message string, keyvals ...interface{}) {
	l.Log(log.InfoLevel, message, keyvals...)
}

func (l *RateLimitedLogger) Debug(message string, keyvals ...interface{}) {
	l.Log(log.DebugLevel, message, keyvals...)
}

func (l *RateLimitedLogger) Error(message string, keyvals ...interface{}) {
	l.Log(log.ErrorLevel, message, keyvals...)
}

func (l *RateLimitedLogger) Warn(message string, keyvals ...interface{}) {
	l.Log(log.WarnLevel, message, keyvals...)
}
