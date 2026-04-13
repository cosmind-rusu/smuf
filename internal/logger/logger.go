package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	infoLogger  = log.New(os.Stdout, "", 0)
	errorLogger = log.New(os.Stderr, "", 0)
)

func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func Info(format string, args ...any) {
	infoLogger.Println(fmt.Sprintf("[%s] INFO  %s", timestamp(), fmt.Sprintf(format, args...)))
}

func Error(format string, args ...any) {
	errorLogger.Println(fmt.Sprintf("[%s] ERROR %s", timestamp(), fmt.Sprintf(format, args...)))
}

func Fatal(format string, args ...any) {
	errorLogger.Println(fmt.Sprintf("[%s] FATAL %s", timestamp(), fmt.Sprintf(format, args...)))
	os.Exit(1)
}
