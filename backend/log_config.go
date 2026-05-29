package main

import (
	"io"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// SetupLogging configures logging for the application
func SetupLogging() {
	// Create logs directory if it doesn't exist
	os.MkdirAll("logs", os.ModePerm)

	// Create log file with date
	logFileName := "logs/" + time.Now().Format("2006-01-02") + ".log"
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	// Set up logging
	gin.DefaultWriter = io.MultiWriter(logFile, os.Stdout)
	gin.DisableConsoleColor()
}
