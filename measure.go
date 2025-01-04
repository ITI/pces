package pces

import (
	"encoding/json"
	"fmt"
	"strconv"
	"gopkg.in/yaml.v3"
	"os"
)

type MeasureStep struct {
	Time     float64 // number of seconds since Measure trace started
	DstDev   string  // target device after network step
	DstCP    string  // target comp pattern after network step
	DstLabel string  // target function label after network step
	Latency  string // latency across network hop just completed
	// Bndwdth float64   // observed free bandwidth across network step
	// PrPass  float64   // estimated probability of successful passage across network step
}

type TerseMeasureStep struct {
	Time float64
	Latency string
}

type MeasureRecord struct {
	Start        float64
	Index		 int
	SrcDev       string
	SrcCP        string
	SrcLabel     string
	MeasureSteps []*MeasureStep
}

type TerseMeasureRecord struct {
	Start        float64
	Index		 int
	MeasureSteps []*TerseMeasureStep
}

var measureID2Name map[int]string = make(map[int]string)

func createMeasureRecord(cpfi *CmpPtnFuncInst, msrName string, msrCalls int, srtTime float64, msg *CmpPtnMsg) {
	srtTime = timeInUnits(srtTime, TimeUnits)
	prec := new(MeasureRecord)
	trsPrec := new(TerseMeasureRecord)

	prec.MeasureSteps = make([]*MeasureStep, 0)
	trsPrec.MeasureSteps = make([]*TerseMeasureStep, 0)

	prec.Start = srtTime
	trsPrec.Start = srtTime

	prec.Index = msrCalls
	trsPrec.Index = msrCalls

	prec.SrcCP = CmpPtnInstByID[cpfi.CPID].Name
	prec.SrcDev = cpfi.Host
	prec.SrcLabel = cpfi.Label
	prbID := nxtMeasureID()
	measureID2Name[prbID] = msrName
	MeasureStats[prbID] = prec
	TerseMeasureStats[prbID] = trsPrec

	msg.MeasureID = prbID
	MeasureID2Name[prbID] = msrName
}

func recordMeasure(MeasureID int, time float64, dstDev, dstCP, dstLabel string) {
	time = timeInUnits(time, TimeUnits)
	msrStep := new(MeasureStep)
	trsStep := new(TerseMeasureStep)
	msrStep.Time = time
	trsStep.Time = time
	msrStep.DstDev = dstDev
	msrStep.DstCP = dstCP
	msrStep.DstLabel = dstLabel
	pstats := MeasureStats[MeasureID]
	trsPstats := TerseMeasureStats[MeasureID]
	pstats.MeasureSteps = append(pstats.MeasureSteps, msrStep)
	trsPstats.MeasureSteps = append(trsPstats.MeasureSteps, trsStep)
}

func endMeasure(MeasureID int, time float64, dstDev, dstCP, dstLabel string) {
	// make a last record ?
	time = timeInUnits(time, TimeUnits)
	pstats := MeasureStats[MeasureID]
	trsPstats := TerseMeasureStats[MeasureID]

	startTime := pstats.Start
	prec := new(PerfRecord)
	trsPrec := new(TersePerfRecord)

	prec.Latency = timeInUnitsStr(time - pstats.Start, TimeUnits)
	trsPrec.Latency = prec.Latency

	prec.MeasureName = MeasureID2Name[MeasureID]
	trsPrec.MeasureName = prec.MeasureName

	prec.Index = pstats.Index
	trsPrec.Index = prec.Index

	prec.SrcDev = pstats.SrcDev
	prec.SrcCP = pstats.SrcCP
	prec.SrcLabel = pstats.SrcLabel

	prec.Waypoints = make([]MeasureStep, len(pstats.MeasureSteps))
	trsPrec.Waypoints = make([]TerseMeasureStep, len(trsPstats.MeasureSteps))

	prec.DstDev = dstDev
	prec.DstCP = dstCP
	prec.DstLabel = dstLabel

	// var bndwdth float64 = 0.0
	// var prPass  float64 = 1.0
	if len(pstats.MeasureSteps) > 0 {
		// bndwdth = pstats.MeasureSteps[0].Bndwdth
		// prPass  = pstats.MeasureSteps[0].PrPass
		prec.Waypoints[0] = *pstats.MeasureSteps[0]
		trsPrec.Waypoints[0] = *trsPstats.MeasureSteps[0]

		prec.Waypoints[0].Time -= startTime
		trsPrec.Waypoints[0].Time = prec.Waypoints[0].Time

		for idx := 1; idx < len(pstats.MeasureSteps); idx++ {
			prstep := pstats.MeasureSteps[idx]
			trsPrstep := trsPstats.MeasureSteps[idx]
			// bndwdth = math.Min(bndwdth, prstep.Bndwdth)
			// prPass *= prstep.PrPass
			prec.Waypoints[idx] = *prstep
			trsPrec.Waypoints[idx] = *trsPrstep

			prec.Waypoints[idx].Time -= startTime
			trsPrec.Waypoints[idx].Time = prec.Waypoints[idx].Time
		}
	}
	// prec.Bndwdth = bndwdth
	// prec.PrLoss = 1.0-prPass

	precVar := *prec
	trsPrecVar := *trsPrec

	if TerseMeasured == nil {
		TerseMeasured = new(TerseMsrData)
		TerseMeasured.Measurements = make([]TersePerfRecord,0)
	}	

	if Measured == nil {
		Measured = new(MsrData)
		Measured.Measurements = make([]PerfRecord,0)
	}	

	Measured.Measurements = append(Measured.Measurements, precVar)
	TerseMeasured.Measurements = append(TerseMeasured.Measurements, trsPrecVar)
}

// PerfRecord stores the Measured information about the packet Measure and the
// flow Measure from SrcName CP to DstName CP
type PerfRecord struct {
	MeasureName string
	Index		int
	SrcDev      string
	SrcCP       string
	SrcLabel    string
	DstDev      string
	DstCP       string
	DstLabel    string
	Latency     string
	// Bndwdth float64
	// PrLoss float64
	Waypoints []MeasureStep
}

type TersePerfRecord struct {
	MeasureName string
	Index		int
	Latency     string
	Waypoints []TerseMeasureStep
}


// MsrData is the structure which holds the Measured output,
// and whose serialization is written to file when finished
type MsrData struct {
	Exprmnt string
	Measurements []PerfRecord
}

type TerseMsrData struct {
	Exprmnt string
	Measurements []TersePerfRecord
}


var numMeasures int = 0

func nxtMeasureID() int {
	numMeasures += 1
	return numMeasures
}

// Serialize creates a string from an instance of the MsrData structure
func (msrd *MsrData) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error
	if useYAML {
		bytes, merr = yaml.Marshal(*msrd)
	} else {
		bytes, merr = json.Marshal(*msrd)
	}
	if merr != nil {
		panic(fmt.Errorf("measurement marshalling error"))
	}
	outputStr := string(bytes[:])
	return outputStr, nil
}

// Serialize creates a string from an instance of the MsrData structure
func (tmsrd *TerseMsrData) Serialize(useYAML bool) (string, error) {
	var bytes []byte
	var merr error
	if useYAML {
		bytes, merr = yaml.Marshal(*tmsrd)
	} else {
		bytes, merr = json.Marshal(*tmsrd)
	}
	if merr != nil {
		panic(fmt.Errorf("measurement marshalling error"))
	}
	outputStr := string(bytes[:])
	return outputStr, nil
}

var Measured *MsrData
var TerseMeasured *TerseMsrData

// MeasureStats and TerseMeasureStats hold measurements made during the run
var MeasureStats map[int]*MeasureRecord = make(map[int]*MeasureRecord)
var TerseMeasureStats map[int]*TerseMeasureRecord = make(map[int]*TerseMeasureRecord)

var MeasureID2Name map[int]string = make(map[int]string)

// SaveMeasureResults is called by the experiment control to save the Measurements
// into the file whose name is passed
func SaveMeasureResults(msrFileName string, exprmntName string, useYAML bool) {

	if Measured == nil || len(Measured.Measurements) == 0{
		return
	}

	Measured.Exprmnt = exprmntName
	TerseMeasured.Exprmnt = exprmntName

	var outputStr string
	if MsrVerbose {
		// simulation is done, write out the Measurements
		outputStr, _ = Measured.Serialize(useYAML)
	} else {
		// simulation is done, write out the Measurements
		outputStr, _ = TerseMeasured.Serialize(useYAML)
	}

	f, cerr := os.Create(msrFileName)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(outputStr)
	if werr != nil {
		panic(werr)
	}
	f.Close()
}

func timeInUnitsStr(t float64, TimeUnits string) string {
	switch TimeUnits {
		case "sec":
			return strconv.FormatFloat(t, 'f', -1, 64)+" (sec)"
		case "msec":
			return strconv.FormatFloat(t, 'f', -1, 64)+" (msec)"
		case "musec":
			return strconv.FormatFloat(t, 'f', -1, 64)+" (musec)"
		case "nsec":
			return strconv.FormatFloat(t, 'f', -1, 64)+" (nsec)"
	}
	return ""
}
 
func timeInUnits(t float64, TimeUnits string) float64 {
	switch TimeUnits {
		case "sec":
			return t
		case "msec":
			return t*1e3
		case "musec":
			return t*1e6
		case "nsec":
			return t*1e9
	}
	return 0.0
} 
