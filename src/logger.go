package main

import (
	"io"
	"log"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// logger is a global instance of the custom logger
var logger *customLogger

// customLogger wraps the standard log.Logger with additional functionality
type customLogger struct {
	*log.Logger
	logFile *lumberjack.Logger
}

// initLogger initializes the global logger based on configuration
func initLogger(cfg *loggerConfig) {
	// If config is nil or no file is specified, create a default config
	if cfg == nil || cfg.File == "" {
		cfg = &loggerConfig{
			File:           "peerapi-agent.log",
			MaxSize:        10,
			MaxBackups:     10,
			MaxAge:         30,
			Compress:       true,
			ConsoleLogging: true,
		}
	}

	// Set default values if not provided
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 10
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 10
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 30
	}

	// Create the lumberjack logger for file output with rotation
	logFile := &lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    cfg.MaxSize, // megabytes
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge, // days
		Compress:   cfg.Compress,
	}

	// Define the writers for the logger
	var writers []io.Writer
	writers = append(writers, logFile)

	// Add console output if enabled
	if cfg.ConsoleLogging {
		writers = append(writers, os.Stdout)
	}

	// Create multi-writer for output to both file and console if needed
	multiWriter := io.MultiWriter(writers...)

	// Create the custom logger
	logger = &customLogger{
		Logger:  log.New(multiWriter, "", log.LstdFlags),
		logFile: logFile,
	}

	// Replace the standard logger's output with our multi-writer
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags)
}

// Close closes the log file
func (l *customLogger) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

// Printf is a wrapper for log.Printf
func (l *customLogger) Printf(format string, v ...any) {
	l.Logger.Printf(format, v...)
}

// Println is a wrapper for log.Println
func (l *customLogger) Println(v ...any) {
	l.Logger.Println(v...)
}

// Fatalf is a wrapper for log.Fatalf
func (l *customLogger) Fatalf(format string, v ...any) {
	l.Logger.Fatalf(format, v...)
}

// Fatal is a wrapper for log.Fatal
func (l *customLogger) Fatal(v ...any) {
	l.Logger.Fatal(v...)
}

// Panicf is a wrapper for log.Panicf
func (l *customLogger) Panicf(format string, v ...any) {
	l.Logger.Panicf(format, v...)
}

// Panic is a wrapper for log.Panic
func (l *customLogger) Panic(v ...any) {
	l.Logger.Panic(v...)
}
