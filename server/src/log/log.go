package log

import (
    "fmt"
    "os"
    "runtime"
    "strings"
    "sync"
    "time"
)

type LogLevel int
const (
    TraceLevel LogLevel = iota
    DebugLevel
    InfoLevel
    WarnLevel
    ErrorLevel
    FatalLevel
)

var logDirectory string
var serverlog *os.File
var serverchan chan string
var days <-chan time.Time // produces a Time on the day
var firstRollover bool
var done chan struct{}
var level LogLevel
var overrides map[string]LogLevel

var stoppingMux sync.Mutex = sync.Mutex{}

func Init(ld string, l LogLevel, o ...map[string]LogLevel) {
    logDirectory = ld
    serverlog = openLog()

    level = l
    serverchan = make(chan string, 10)
    done = make(chan struct{}, 1)
    if len(o) > 0 {
        overrides = o[0]
    }

    // We don't worry about rollover when running tests.
    if ld != "/tmp" {
        firstRollover = true
        firstTick := make(chan time.Time, 1)
        days = func() <-chan time.Time { return firstTick }()
        go firstDayTick(firstTick)
    }

    go runServerlog()
}

func Stop(s string, r interface{}) {
    // This is never unlocked.  The first goroutine to panic and end up here
    // gets the lock, all others pile up until the first caller inevitably
    // kills the application
    stoppingMux.Lock()

    Fatal("log.Stop first call: '%s':%s", s, r)
    close(serverchan)
    <-done
}

func Fatal(msg string, fargs ...interface{}) {
    p := getPkg()
    if level <= FatalLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[FATAL] (%s) %s", p, msg), fargs...)
    } else if l, ok := overrides[p]; ok && l <= FatalLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[FATAL] (%s) %s", p, msg), fargs...)
    }
}

func Error(msg string, fargs ...interface{}) {
    p := getPkg()
    if level <= ErrorLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[ERROR] (%s) %s", p, msg), fargs...)
    } else if l, ok := overrides[p]; ok && l <= ErrorLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[ERROR] (%s) %s", p, msg), fargs...)
    }
}

func Warn(msg string, fargs ...interface{}) {
    p := getPkg()
    if level <= WarnLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[WARN] (%s) %s", p, msg), fargs...)
    } else if l, ok := overrides[p]; ok && l <= WarnLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[WARN] (%s) %s", p, msg), fargs...)
    }
}

func Info(msg string, fargs ...interface{}) {
    p := getPkg()
    if level <= InfoLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[INFO] (%s) %s", p, msg), fargs...)
    } else if l, ok := overrides[p]; ok && l <= InfoLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[INFO] (%s) %s", p, msg), fargs...)
    }
}

func Debug(msg string, fargs ...interface{}) {
    p := getPkg()
    if level <= DebugLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[DEBUG] (%s) %s", p, msg), fargs...)
    } else if l, ok := overrides[p]; ok && l <= DebugLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[DEBUG] (%s) %s", p, msg), fargs...)
    }
}

func Trace(msg string, fargs ...interface{}) {
    p := getPkg()
    if level <= TraceLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[TRACE] (%s) %s", p, msg), fargs...)
    } else if l, ok := overrides[p]; ok && l <= TraceLevel {
        serverchan <-fmt.Sprintf(fmt.Sprintf("[TRACE] (%s) %s", p, msg), fargs...)
    }
}

func runServerlog() {
    for {
        select {
        case m, ok := <-serverchan:
            if ok {
                log(serverlog, m)
            } else {
                serverlog.Sync()
                close(done)
                return
            }
        case <-days:
            if firstRollover {
                days = time.NewTicker(86400 * time.Second).C
                firstRollover = false
            }
            rollLog()
        }
    }
}

func firstDayTick(x chan time.Time) {
    now := time.Now()
    dayEnd := time.Date(now.Year(), now.Month(), now.Day(),
        0, 0, 0, 0, now.Location())
    dayEnd = dayEnd.Add(24 * time.Hour)
    sleepTime := dayEnd.Sub(now)
    Debug("Log rollover initialization: now: '%s' dayEnd: '%s', sleepTime: '%s'",
        now, dayEnd, sleepTime)
    time.Sleep(sleepTime)
    x <-time.Now()
}

func rollLog() {
    serverlog.Close()
    serverlog = openLog()
}

func openLog() *os.File {
    f, err := os.OpenFile(
        fmt.Sprintf("%s/server.log.%s", logDirectory, logSuffix()),
        os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        panic(fmt.Sprintf("Unable to open serverlog in %s for writing: %s",
            logDirectory, err))
    }
    return f
}

// date +'%Y%m%d'
func logSuffix() string {
    return time.Now().Format("20060102")
}

func getPkg() string {
	pc, _, _, ok := runtime.Caller(2)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
        name := details.Name()
        return name[strings.LastIndex(name, "/")+1:strings.Index(name, ".")]
	}
    return ""
}

func log(logfile *os.File, msg string) {
    logfile.WriteString(fmt.Sprintf("%s %s\n",
        time.Now().Format("15:04:05.000Z"),
        strings.ReplaceAll(msg, "\n", "\\n")))
}
