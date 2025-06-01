package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

type Logger struct {
	logger       *log.Logger
	level        LogLevel
	file         *os.File
	enableCaller bool
	debugMode    bool
}

// グローバルロガーインスタンス
var globalLogger *Logger

// IsDebugEnabled デバッグモードが有効かどうかを確認
func IsDebugEnabled() bool {
	if globalLogger == nil {
		return false
	}
	return globalLogger.debugMode
}

// InitLogger グローバルロガーを初期化
func InitLogger(logPath string, level LogLevel, debugMode bool) error {
	logger, err := NewFileOnlyLogger(logPath, level)
	if err != nil {
		return err
	}
	logger.debugMode = debugMode
	globalLogger = logger
	return nil
}

// InitFileOnlyLogger ファイル専用グローバルロガーを初期化
func InitFileOnlyLogger(logPath string, level LogLevel, debugMode bool) error {
	logger, err := NewFileOnlyLogger(logPath, level)
	if err != nil {
		return err
	}
	logger.debugMode = debugMode
	globalLogger = logger
	return nil
}

// GetLogger グローバルロガーを取得
func GetLogger() *Logger {
	return globalLogger
}

// CloseLogger グローバルロガーを閉じる
func CloseLogger() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}

// グローバル関数群
func Debug(format string, args ...interface{}) {
	if globalLogger != nil && globalLogger.debugMode {
		globalLogger.Debug(format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(format, args...)
	}
}

func Fatal(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Fatal(format, args...)
	}
}

// NewLogger creates a new logger instance
func NewLogger(logPath string, level LogLevel) (*Logger, error) {
	// ログディレクトリを作成
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// ログファイルを開く（追記モード）
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// マルチライター（ファイル + 標準出力）
	multiWriter := io.MultiWriter(file, os.Stdout)

	logger := &Logger{
		logger:       log.New(multiWriter, "", 0),
		level:        level,
		file:         file,
		enableCaller: true,
		debugMode:    false,
	}

	return logger, nil
}

// NewFileOnlyLogger creates a logger that only writes to file
func NewFileOnlyLogger(logPath string, level LogLevel) (*Logger, error) {
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := &Logger{
		logger:       log.New(file, "", 0),
		level:        level,
		file:         file,
		enableCaller: true,
		debugMode:    false,
	}

	return logger, nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// EnableCaller enables/disables caller information in logs
func (l *Logger) EnableCaller(enable bool) {
	l.enableCaller = enable
}

// SetDebugMode enables/disables debug mode
func (l *Logger) SetDebugMode(enable bool) {
	l.debugMode = enable
}

// IsDebugMode returns whether debug mode is enabled
func (l *Logger) IsDebugMode() bool {
	return l.debugMode
}

// log is the internal logging function
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelStr := levelNames[level]

	var caller string
	if l.enableCaller {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			caller = fmt.Sprintf(" [%s:%d]", filepath.Base(file), line)
		}
	}

	message := fmt.Sprintf(format, args...)
	logLine := fmt.Sprintf("%s [%s]%s %s", timestamp, levelStr, caller, message)

	l.logger.Println(logLine)

	// FATAL レベルの場合はプログラムを終了
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message (only if debug mode is enabled)
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.debugMode {
		l.log(DEBUG, format, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message and exits the program
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}
