// Code generated by "stringer -type=JobImplication"; DO NOT EDIT

package worker

import "fmt"

const _JobImplication_name = "UnknownHaltedFinishedAborted"

var _JobImplication_index = [...]uint8{0, 7, 13, 21, 28}

func (i JobImplication) String() string {
	if i >= JobImplication(len(_JobImplication_index)-1) {
		return fmt.Sprintf("JobImplication(%d)", i)
	}
	return _JobImplication_name[_JobImplication_index[i]:_JobImplication_index[i+1]]
}
