package pces
import (
	"github.com/iti/evt/vrtime"
	"strconv"
	"github.com/iti/mrnes"
	"gopkg.in/yaml.v3"
)


var trtToStr map[mrnes.TraceRecordType]string = map[mrnes.TraceRecordType]string{mrnes.NetworkType:"network",mrnes.CmpPtnType:"cp"}
var TraceMgr *mrnes.TraceManager

// CPTrace saves information about the visitation of a message to some point in the computation pattern portion of the simulation.
// saved for post-run analysis
type CPTrace struct {
	Time      float64 // time in float64
	Ticks     int64   // ticks variable of time
	Priority  int64   // priority field of time-stamp
	ExecID    int     // integer identifier identifying the chain of traces this is part of
	ObjID     int     // integer id for object being referenced
	Op        string  // "start", "stop", "enter", "exit"
	CPM		  string  // serialization of CmpPtnMsg
}

func (cpt *CPTrace) TraceType() mrnes.TraceRecordType {
	return mrnes.CmpPtnType
}

func (cpt *CPTrace) Serialize() string {
	if !useTrace {
		return ""
	}

	var bytes []byte
	var merr error

	bytes, merr = yaml.Marshal(*cpt)

	if merr != nil {
		panic(merr)
	}
	return string(bytes[:])
}

// AddCPTrace creates a record of the trace using its calling arguments, and stores it
func AddCPTrace(tm *mrnes.TraceManager, flag bool, vrt vrtime.Time, execID int, objID int, op string, cpm *CmpPtnMsg) {

	if !flag || !useTrace {
		return
	}
	
	cpt := new(CPTrace)
	cpt.Time = vrt.Seconds()
	cpt.Ticks = vrt.Ticks()
	cpt.Priority = vrt.Pri()
	if cpm != nil {
		cpt.ExecID   = cpm.ExecID
	} else {
		cpt.ExecID   = 0
	}
	cpt.ObjID = objID
	cpt.Op = op
	if cpm != nil {
		cpt.CPM = cpm.Serialize()
	}
	cptStr := cpt.Serialize()

	timeInStr := strconv.FormatFloat(cpt.Time, 'f', -1, 64)

	trcInst := mrnes.TraceInst{TraceTime: timeInStr, TraceType:"CP", TraceStr: cptStr}
	tm.AddTrace(vrt, cpt.ExecID, trcInst)
}
