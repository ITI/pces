package pces

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type measureStep struct {
	Time     float64 // number of seconds since measure trace started
	DstDev   string  // target device after network step
	DstCP    string  // target comp pattern after network step
	DstLabel string  // target function label after network step
	Latency  float64 // latency across network hop just completed
	// Bndwdth float64   // observed free bandwidth across network step
	// PrPass  float64   // estimated probability of successful passage across network step
}

type measureRecord struct {
	Start        float64
	SrcDev       string
	SrcCP        string
	SrcLabel     string
	MeasureSteps []*measureStep
}

func createMeasureRecord(cpfi *CmpPtnFuncInst, msrName string, srtTime float64, msg *CmpPtnMsg) *measureRecord {
	prec := new(measureRecord)
	prec.MeasureSteps = make([]*measureStep, 0)
	prec.Start = srtTime
	prec.SrcCP = CmpPtnInstByID[cpfi.CPID].Name
	prec.SrcDev = cpfi.Host
	prec.SrcLabel = cpfi.Label
	prbID := nxtMeasureID()
	measureStats[prbID] = prec
	msg.MeasureID = prbID
	measureID2Name[prbID] = msrName
	return prec
}

func recordMeasure(measureID int, time float64, dstDev, dstCP, dstLabel string) {
	msrStep := new(measureStep)
	msrStep.Time = time
	msrStep.DstDev = dstDev
	msrStep.DstCP = dstCP
	msrStep.DstLabel = dstLabel
	pstats := measureStats[measureID]
	pstats.MeasureSteps = append(pstats.MeasureSteps, msrStep)
}

func endMeasure(measureID int, time float64, dstDev, dstCP, dstLabel string) {
	// make a last record ?
	// recordMeasure(measureID int, time float64, dstDev, dstCP, dstLabel string) {

	pstats := measureStats[measureID]
	startTime := pstats.Start
	prec := new(PerfRecord)

	prec.Latency = time - pstats.Start
	prec.MeasureName = measureID2Name[measureID]
	prec.SrcDev = pstats.SrcDev
	prec.SrcCP = pstats.SrcCP
	prec.SrcLabel = pstats.SrcLabel
	prec.Waypoints = make([]measureStep, len(pstats.MeasureSteps))
	prec.DstDev = dstDev
	prec.DstCP = dstCP
	prec.DstLabel = dstLabel

	// var bndwdth float64 = 0.0
	// var prPass  float64 = 1.0
	if len(pstats.MeasureSteps) > 0 {
		// bndwdth = pstats.MeasureSteps[0].Bndwdth
		// prPass  = pstats.MeasureSteps[0].PrPass
		prec.Waypoints[0] = *pstats.MeasureSteps[0]
		prec.Waypoints[0].Time -= startTime
		for idx := 1; idx < len(pstats.MeasureSteps); idx++ {
			prstep := pstats.MeasureSteps[idx]
			// bndwdth = math.Min(bndwdth, prstep.Bndwdth)
			// prPass *= prstep.PrPass
			prec.Waypoints[idx] = *prstep
			prec.Waypoints[idx].Time -= startTime
		}
	}
	// prec.Bndwdth = bndwdth
	// prec.PrLoss = 1.0-prPass

	precVar := *prec
	Measured.Measurements = append(Measured.Measurements, precVar)
}

// PerfRecord stores the Measured information about the packet measure and the
// flow measure from SrcName CP to DstName CP
type PerfRecord struct {
	MeasureName string
	SrcDev      string
	SrcCP       string
	SrcLabel    string
	DstDev      string
	DstCP       string
	DstLabel    string
	Latency     float64
	// Bndwdth float64
	// PrLoss float64
	Waypoints []measureStep
}

// MsrData is the structure which holds the Measured output,
// and whose serialization is written to file when finished
type MsrData struct {
	Measurements []PerfRecord
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

var Measured *MsrData

// execID is the map key
var measureStats map[int]*measureRecord = make(map[int]*measureRecord)

var measureID2Name map[int]string = make(map[int]string)

// SaveMeasureResults is called by the experiment control to save the measurements
// into the file whose name is passed
func SaveMeasureResults(msrFileName string, useYAML bool) {

	if Measured == nil {
		return
	}

	// simulation is done, write out the Measurements
	outputStr, _ := Measured.Serialize(useYAML)

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
