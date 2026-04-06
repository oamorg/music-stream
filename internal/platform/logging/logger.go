package logging

import (
	"log"
	"os"
)

func New(env string) *log.Logger {
	prefix := "[" + env + "] "
	return log.New(os.Stdout, prefix, log.Ldate|log.Ltime|log.Lmicroseconds|log.LUTC)
}
