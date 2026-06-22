package logger

import (
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/op/go-logging"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultLogMaxSizeMB  = 10
	defaultLogMaxBackups = 3
	defaultLogMaxAgeDays = 7
)

var (
	logger        *logging.Logger
	loggerMu      sync.Mutex
	activeLogFile *lumberjack.Logger
)

func init() {
	InitLogger(logging.INFO)
}

func InitLogger(level logging.Level) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	if activeLogFile != nil {
		activeLogFile.Close()
		activeLogFile = nil
	}

	format := logging.MustStringFormatter(
		`%{time:2006/01/02 15:04:05} %{level} - %{message}`,
	)
	newLogger := logging.MustGetLogger("x-ui")
	backend := logging.NewLogBackend(logWriter(), "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "")
	newLogger.SetBackend(backendLeveled)

	logger = newLogger
}

func logWriter() io.Writer {
	logFile := os.Getenv("XUI_LOG_FILE")
	if logFile == "" {
		return os.Stderr
	}
	activeLogFile = &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    envInt("XUI_LOG_MAX_SIZE_MB", defaultLogMaxSizeMB),
		MaxBackups: envInt("XUI_LOG_MAX_BACKUPS", defaultLogMaxBackups),
		MaxAge:     envInt("XUI_LOG_MAX_AGE_DAYS", defaultLogMaxAgeDays),
		Compress:   false,
	}
	return io.MultiWriter(os.Stderr, activeLogFile)
}

func envInt(name string, defaultValue int) int {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return defaultValue
	}
	return parsed
}

func Debug(args ...interface{}) {
	logger.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func Info(args ...interface{}) {
	logger.Info(args...)
}

func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Warning(args ...interface{}) {
	logger.Warning(args...)
}

func Warningf(format string, args ...interface{}) {
	logger.Warningf(format, args...)
}

func Error(args ...interface{}) {
	logger.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}
