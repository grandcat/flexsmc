package statistics

import (
	"bufio"
	"flag"
	"log"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

// Extract package + function name.
// E.g. `github.com/grandcat/flexsmc/benchmark/timelog.Test_enter` -> `timelog.Test_enter`
var extractFnName = regexp.MustCompile(`^.*\/(.*)$`)

var timeLogger = newTimeLog()

type timeLog struct {
	writer *bufio.Writer
	logger *log.Logger

	setPrefix   string //< filled via flags
	granularity Level  //< filled via flags by default
}

func newTimeLog() timeLog {
	// Prepare output files.
	f, err := os.OpenFile("stats_log.out", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		panic("Could not open file for writing.")
	}
	w := bufio.NewWriter(f)
	// Prepare logger.
	logger := log.New(w, "", 0)

	tl := timeLog{
		writer: w,
		logger: logger,
	}
	return tl
}

// output forms a CSV entry with all items separated by a commata, and sends
// it to the writer.
func (tl *timeLog) output(id, funcName string, d time.Duration, args ...string) {
	tl.writer.WriteString(tl.setPrefix)
	tl.writer.WriteByte(',')
	tl.writer.WriteString(id)
	tl.writer.WriteByte(',')
	tl.writer.WriteString(funcName)
	tl.writer.WriteByte(',')
	tl.writer.WriteString(strconv.FormatInt(d.Nanoseconds(), 10))
	for _, a := range args {
		tl.writer.WriteByte(',')
		tl.writer.WriteString(a)
	}
	tl.writer.WriteByte('\n')
}

func (tl *timeLog) printf(format string, args ...interface{}) {
	tl.logger.Printf(format, args)
}

func (tl *timeLog) flush() {
	tl.writer.Flush()
}

func init() {
	flag.StringVar(&timeLogger.setPrefix, "stats_id", "", "Set the prefix identifier to distinguish different test configurations")
	flag.Var(&timeLogger.granularity, "stats_granularity", "granularity")
}

func SetGranularity(lev Level) {
	timeLogger.granularity.set(lev)
}

func GracefulFlush() {
	timeLogger.flush()
}

type Level int32

// get returns the value of the granularity level.
func (l *Level) get() Level {
	return Level(atomic.LoadInt32((*int32)(l)))
}

// set sets the value of the granularity level.
func (l *Level) set(lev Level) {
	atomic.StoreInt32((*int32)(l), int32(lev))
}

// Get is part of the flag.Value interface.
func (l *Level) Get() interface{} {
	return *l
}

// Set is part of the flag.Value interface.
func (l *Level) Set(val string) error {
	v, err := strconv.Atoi(val)
	if err != nil {
		return err
	}
	l.set(Level(v))
	return nil
}

// String is part of the flag.Value interface.
func (l *Level) String() string {
	return strconv.FormatInt(int64(*l), 10)
}

type Track bool

func G(lev Level) Track {
	if timeLogger.granularity.get() >= lev {
		return Track(true)
	}
	return Track(false)
}

func StartTrack() time.Time {
	return time.Now()
}

func (t Track) End(identifier interface{}, start time.Time, logArgs ...string) {
	if t {
		elapsed := time.Since(start)

		// Identify object.
		var strID string
		switch id := identifier.(type) {
		case string:
			strID = id
		default:
			// Probably a protobuf object (e.g. job phase).
			// Use this type's string instead.
			strID = reflect.TypeOf(id).String()
		}

		// Extract package and function name.
		var funcName string
		if true {
			pc, _, _, _ := runtime.Caller(1)
			funcObj := runtime.FuncForPC(pc)
			funcName = extractFnName.ReplaceAllString(funcObj.Name(), "$1")
		}
		timeLogger.output(strID, funcName, elapsed, logArgs...)
	}
}
