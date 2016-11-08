package logs

import "github.com/golang/glog"

const (
	important glog.Level = iota //< 0
	info
	detailed
	verbose
	trace
)

var (
	// C sets verbosity for important messages
	C glog.Verbose
	// I sets verbosity for info log
	I glog.Verbose
	// V sets verbosity for detailed logs or debug messages
	V glog.Verbose
	// VV sets verbosity for a high need of verbosity
	VV glog.Verbose
	// T sets verbosity for trace logs
	T glog.Verbose
)

// Map all important log functions to glog equivalents.
var (
	Info   = glog.Info
	Infof  = glog.Infof
	Infoln = glog.Infoln

	Warning   = glog.Warning
	Warningf  = glog.Warningf
	Warningln = glog.Warningln

	Error   = glog.Error
	Errorf  = glog.Errorf
	Errorln = glog.Errorln

	Fatal   = glog.Fatal
	Fatalf  = glog.Fatalf
	Fatalln = glog.Fatalln
)

// Init must be called prior to any call to C, I, V, VV or T.
func Init() {
	glog.Infoln("Init glog logging")
	// Assign to runtime to make sure that glog parsed required cmd flags.
	// Otherwise, all logs will be discarded.
	C = glog.V(0)
	I = glog.V(1)
	V = glog.V(2)
	VV = glog.V(3)
	T = glog.V(4)
}
