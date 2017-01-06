package statistics

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Extract package + function name.
// E.g. `github.com/grandcat/flexsmc/benchmark/timelog.Test_enter` -> `timelog.Test_enter`
var extractFnName = regexp.MustCompile(`^.*\/(.*)$`)

var timeLogger *timeLog

type timeLog struct {
	fileName string
	f        *os.File
	wr       *bufio.Writer

	setID       string
	granularity Level //< filled via flags by default

	mu sync.Mutex
}

func (tl *timeLog) newLogfile(filePrefix string) {
	// Prepare unique output file in temp folder.
	f, err := ioutil.TempFile("", filePrefix+datePrefix())
	if err != nil {
		panic("Could not open file for writing.")
	}
	// Prepare or switch buffered writer.
	tl.mu.Lock()
	if tl.wr == nil {
		tl.f = f
		tl.wr = bufio.NewWriter(f)
	} else {
		tl.wr.Flush()
		tl.wr.Reset(f)

		tl.f.Close()
		tl.f = f
	}
	tl.mu.Unlock()
}

func datePrefix() string {
	t := time.Now()
	return fmt.Sprintf("stats.log.%04d%02d%02d-%02d%02d%02d.tmp",
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
	)
}

func (tl *timeLog) updateSetID(s string) {
	tl.mu.Lock()
	tl.setID = s
	tl.mu.Unlock()
}

// output forms a CSV entry with all items separated by a commata, and sends
// it to the writer.
func (tl *timeLog) output(id, funcName string, d time.Duration, args ...string) {
	// Normally, the whole function should be protected due to the writer.
	// In this case, the writer object is set only on initialization. Further, we
	// assume there are no concurrent outputs.
	// So, it should be safe to do so.
	tl.mu.Lock()
	tl.wr.WriteString(tl.setID)
	tl.mu.Unlock()
	tl.wr.WriteByte(',')
	tl.wr.WriteString(id)
	tl.wr.WriteByte(',')
	tl.wr.WriteString(funcName)
	tl.wr.WriteByte(',')
	tl.wr.WriteString(strconv.FormatInt(d.Nanoseconds(), 10))
	for _, a := range args {
		tl.wr.WriteByte(',')
		tl.wr.WriteString(a)
	}
	tl.wr.WriteByte('\n')
}

func (tl *timeLog) flush() {
	tl.wr.Flush()
}

func init() {
	timeLogger = new(timeLog)

	flag.StringVar(&timeLogger.setID, "stats_id", "defaultSet", "Set the identifier to distinguish different experiments. Overwriteable during runtime")
	flag.Var(&timeLogger.granularity, "stats_granularity", "granularity")

	timeLogger.newLogfile("stats.log.")
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

func SwitchLog(prefix string) {
	timeLogger.newLogfile(prefix)
}

func UpdateSetID(s string) {
	timeLogger.updateSetID(s)
}

func SetGranularity(lev Level) {
	timeLogger.granularity.set(lev)
}

func GracefulFlush() {
	timeLogger.flush()
}
