package logger

import (
	"fmt"
	"io"
	"log"
	"os"
)

type Logger struct {
	err   *log.Logger
	debug *log.Logger
	info  *log.Logger
	fatal *log.Logger
}

func NewLogger(out io.Writer) *Logger {
	return &Logger{
		err:   log.New(out, "ERROR: ", log.LstdFlags|log.Lshortfile),
		debug: log.New(out, "DEBUG: ", log.LstdFlags|log.Lshortfile),
		info:  log.New(out, "INFO: ", log.LstdFlags),
		fatal: log.New(out, "FATAL: ", log.LstdFlags|log.Lshortfile),
	}

}

func (l *Logger) Info(v ...any)  { l.info.Output(2, fmt.Sprint(v...)) }
func (l *Logger) Debug(v ...any) { l.debug.Output(2, fmt.Sprint(v...)) }
func (l *Logger) Error(v ...any) { l.err.Output(2, fmt.Sprint(v...)) }
func (l *Logger) Fatal(v ...any) {
	l.fatal.Output(2, fmt.Sprint(v...))
	os.Exit(1)
}
