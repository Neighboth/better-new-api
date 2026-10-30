package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

// The client runs for days in the background of the computer. A panic in one
// loop used to end the whole program: the window of the client disappeared
// without a message and the computer stopped serving the network until the user
// started the client again. Every long loop of the client runs through safeGo,
// a panic is written into the log of the client and the coordinator is told
// about it, the other loops keep running.

var logMutex sync.Mutex

// logLine writes one line into the log of the client.
//
// The log of the client is opened for every line: a long running program must
// not hold a file handle that an antivirus of the computer can lock.
func logLine(format string, args ...any) {
	logMutex.Lock()
	defer logMutex.Unlock()
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
	fmt.Print(line)
	file, err := os.OpenFile(logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(line)
	if info, err := file.Stat(); err == nil && info.Size() > 8<<20 {
		// The log of a client that runs for a month would fill the disk of the
		// user: the file is rotated once it reaches a size that a reader can
		// still open.
		_ = os.Rename(logPath(), filepath.Join(storageRoot(), "worker-old.log"))
	}
}

// safeGo runs a function in its own goroutine.
//
// A panic of the function is recovered: the stack is written into the log and
// the coordinator receives the crash report. The client keeps running.
func safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				stack := string(debug.Stack())
				logLine("[CRASH] loop %s panicked: %v", name, recovered)
				logLine("[CRASH] stack: %s", stack)
				reportCrash("loop_panic", fmt.Sprintf("%s: %v", name, recovered), stack, "")
			}
		}()
		fn()
	}()
}

// recoverForever restarts a long loop of the client when it returns.
//
// A loop of the client is a loop of the network and of the files of the user: a
// loop that ends is a loop that stopped serving the network. The loop is
// started again after a pause, a loop that ends at once does not spin.
func recoverForever(name string, loop func()) {
	safeGo(name, func() {
		for {
			startedAt := time.Now()
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						stack := string(debug.Stack())
						logLine("[CRASH] loop %s panicked: %v", name, recovered)
						logLine("[CRASH] stack: %s", stack)
						reportCrash("loop_panic", fmt.Sprintf("%s: %v", name, recovered), stack, "")
					}
				}()
				loop()
			}()
			// A loop that ran for a while failed on a condition of the
			// network, a loop that returned at once is broken.
			pause := 5 * time.Second
			if time.Since(startedAt) > time.Minute {
				pause = time.Second
			}
			logLine("[RESILIENCE] loop %s ended, restarting in %s", name, pause)
			time.Sleep(pause)
		}
	})
}
