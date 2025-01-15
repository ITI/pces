package pces

import (
	"strconv"
	"strings"
	"gopkg.in/yaml.v3"
	"unsafe"
	"hash/fnv"
	"sort"
	"math"
	"os"
)

// Measurement holds a floating point measurement, and the time when the measurement was started
type Measurement struct {
	StartMsr float64	// simulation time when measurement started
	Value float64       // measurement
	MsrName string      // measurement name
}

// MsrGroup holds observations that are in a common class, defined by the user.   
type MsrGroup struct {
	GroupDesc  string          // text description of the group
	GroupType  string		   // one of the constants Latency, Bndwdth, PrLoss  
	ID uint32                  // index for identity
	Measures []Measurement     // list of measurements observed
}

// MsrData holds the information from an MsrGroup, plus more, in a data structure suitable for serialization
type MsrData struct {
	ExprmntName string
	Params map[string]string
	GroupDesc string
	GroupType string
	Measures []string
}

type MsrRoute struct {
	MsrName string
	Visited []int
}

func ComputeMsrGrpHash(msrName string, visits []int) uint32 {
	h := fnv.New32()
	b := []byte(msrName)
	h.Write(b)
	
	for idx:=0; idx< len(visits); idx++ {
		b = IntToByteArray( visits[idx] )
		h.Write(b)
	}
	return h.Sum32()
}	

func CreateMsrRoute(msrName string, execID int) {
	mr := new(MsrRoute)
	mr.MsrName = msrName
	mr.Visited = make([]int,0)
	if MsrRouteByExecID == nil {
		MsrRouteByExecID = make(map[int]*MsrRoute)
	}
	MsrRouteByExecID[execID] = mr	
	MsrMapsInit()
}

func MsrAppendID(execID int, ID int) {
	mr := MsrRouteByExecID[execID]
	mr.Visited = append(mr.Visited, ID)
}

/*
func IntToByteArray(i int64) []byte {
	var b []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh.Len = 8
	sh.Cap = 8
	sh.Data = uintptr(unsafe.Pointer(&i))
	return b[:]
}
*/

func IntToByteArray(i int) []byte {
	s := []int{i}

	// Convert a slice to a byte slice
	data := unsafe.SliceData(s)
	// b := unsafe.Slice((*byte)(unsafe.Pointer(data)), len(s)*unsafe.Sizeof(s[0]))
	b := unsafe.Slice((*byte)(unsafe.Pointer(data)), 8)
	return b	
}

var MsrID2Name map[int]string
var MsrRouteByExecID map[int]*MsrRoute
var MsrGrpByID map[uint32]*MsrGroup

func GetMsrGrp(msrName string, execID int, msrType string, classify func([]int) string) *MsrGroup {
	mr := MsrRouteByExecID[execID]	
	grpID := ComputeMsrGrpHash(msrName, mr.Visited) 
	msrGrp, present := MsrGrpByID[grpID]
	if present {
		return msrGrp
	}
	desc := classify(mr.Visited)	
	msrGrp = CreateMsrGroup(desc, msrType)
	MsrGrpByID[grpID] = msrGrp
	msrGrp.ID = grpID
	return msrGrp
}


func MsrMapsInit() {
	if MsrID2Name == nil {
		MsrID2Name = make(map[int]string)
	}
	if MsrGrpByID == nil {
		MsrGrpByID = make(map[uint32]*MsrGroup)
	}
	if MsrRouteByExecID == nil {
		MsrRouteByExecID = make(map[int]*MsrRoute)
	}
}

// CreateMsrGroup establishes a struct that holds a description of a group of measurements,
// and the measurements themselves
func CreateMsrGroup(desc string, groupType string) *MsrGroup {
	MsrMapsInit()
	msrg := new(MsrGroup)
	msrg.GroupDesc = desc
	msrg.GroupType = groupType
	msrg.Measures = make([]Measurement,0)
	return msrg
}

// CreateMsrData takes the MsrGroup data and puts it into a form suitable for serialization
func (msrg *MsrGroup) CreateMsrData(exprmntName string) *MsrData {
	md := new(MsrData)
	md.ExprmntName = exprmntName
	md.GroupDesc = msrg.GroupDesc
	md.GroupType = msrg.GroupType
	md.Measures = make([]string, len(msrg.Measures))

	for idx:=0; idx<len(msrg.Measures); idx++ {
		md.Measures[idx] = strconv.FormatFloat(msrg.Measures[idx].Value, 'g', -1, 64)
	}
	return md
}

// AddMeasure creates a new Measurement and adds it to the MsrGroup
func (msrg *MsrGroup)AddMeasure(startMsr, measure float64, msrName string) {
    m := new(Measurement)
	m.StartMsr = startMsr
	m.Value = timeInUnits(measure, TimeUnits)
	m.MsrName = msrName
	msrg.Measures = append(msrg.Measures, *m) 
}

var Samples int = 0
var Skip int = 0
var Batch int = 1

// MsrStats computes the mean and variance over the data in the MsrGroup.
// The mean may be computed by batch-means, batch size given, and
// the number of initial samples to skip to avoid initialization bias
func (msrg *MsrGroup) MsrStats(batchmeans bool) (int, float64, float64) {
	var skip, batch int
	if batchmeans {
		skip = Skip
		batch = Batch
	} else {
		skip = 0
		batch = 1
	}

	// if skip and/or batch are larger than data set, nothing
	if batchmeans && len(msrg.Measures) <= Skip*Batch {
		return 0, 0.0, 0.0
	}

	// compute the index range over the non-aggregated samples
	srtIdx := skip*batch
	endIdx := batch*(len(msrg.Measures)/batch)

	n := 0		// number of batch-means samples
	sum := 0.0	// sum of batch-means samples
	sum2 := 0.0	// sum of squares of batch-means samples

	for idx := srtIdx; idx<endIdx; idx += batch {
		avg := 0.0		// total over batch

		// visit every sample in batch
		for jdx := idx; jdx < idx+batch; jdx++ {
			value := msrg.Measures[idx].Value
			avg += value
		}

		avg /= float64(batch)	// compute the batch-mean sample
		sum += avg
		sum2 += avg*avg
		n += 1
	}

	fn := float64(n)		// floating point needed for arithmetic with floats
	mean := sum/fn			// batch-means mean
	sqrMean := sum2/fn		// batch-means avg sample^2

	// results
	return n, mean, math.Sqrt(sqrMean-mean*mean)
}

// MsrRange returns the min, 25% percentile, median, 75% percentile, and maximum value
// of data in the group
func (msrg *MsrGroup) MsrRange() (int, []float64) {

	// create sorted representation of data values
	data := make([]float64, len(msrg.Measures))
	for idx, msr := range msrg.Measures {
		data[idx] = msr.Value
	}
	sort.Float64s(data)


	// quartiles depend on whether we need to split values or not
	var lower, median, upper float64
	if len(data)%4 == 0 {

		// Four equal sized bins
		quarter := len(data)/4
		half    := len(data)/2
		lower = (data[quarter-1]+data[quarter])/2.0
		upper = (data[3*quarter-1]+data[3*quarter])/2.0
		median = (data[half-1]+data[half])/2.0		
	} else if len(data)%2 == 0 {
		// two equal sized bins
		half    := len(data)/2
		quarter := len(data)/4
		
		// the two halfs are both odd 
		lower = data[quarter]
		upper = data[half+quarter]
		median = (data[half-1]+data[half])/2.0		
	} else {	
		// sample size is odd so median is middle element	
		half    := len(data)/2
		median = data[half]

		if half%2 == 0 {	
			quarter := half/2
			lower = data[quarter]
			upper = data[half+quarter]
		} else {
			quarter := half/2
			lower = (data[quarter]+data[quarter+1])/2.0
			upper = (data[half+quarter]+data[half+quarter+1])/2.0
		}
	}
	rtn := []float64{data[0], lower, median, upper, data[len(data)-1]}
	return len(data), rtn
}

var paramCodes []string
var columnHdr []string
var fixedRow  []string
var paramsDict map[string]string
var rowType string

// PrepCSVRow is called at the end of a run to create a header for the experiment's
// csv output file, if needed (actually only needed on the first call), and set up
// other data structures

func (msrg *MsrGroup) PrepCSVRow(csvFileName string, expFileName string, 
		exprmntName string, hdr []string, data []string, rType string) []string {

	paramsDict := make(map[string]string)
	if len(expFileName) > 0 {
		paramsDict = ReturnParamsDict(expFileName, exprmntName)
	}

	// we'll put in the measure name with the samples
	fixedRow  = []string{exprmntName,""}
	rowType = rType

	columnHdr := []string{"Experiment", "MeasureName"}

	// add the user specified headers
	columnHdr = append(columnHdr, hdr...)

	// make room for the user specified data
	for idx:=0; idx<len(hdr); idx++ {
		fixedRow = append(fixedRow, "")
	}

	// copy the user specified data
	for idx := 0; idx<len(data); idx++ {
		fixedRow[idx+2] = data[idx]
	}

	// add the parameters. First order the keys of the parameters
	paramCodes = make([]string, len(paramsDict))
	idx := 0
	for param := range paramsDict {
		paramCodes[idx] = param
		idx += 1
	}
	sort.Strings(paramCodes)

	// add the header and fixed part of the row
	for _, param := range paramCodes {
		columnHdr = append(columnHdr, param)
		fixedRow = append(fixedRow, paramsDict[param])
	}

	// Needed to create fixedRow, but that's done now 
	// if the csvFile exists already the header has been made and written and there is nothing to do
	_, err := os.Stat(csvFileName)	
	if err==nil {
		return []string{}
	}


	// The remainder of the header depends on what type of output is being asked for
	if rType == "Samples" {
		AddOn := []string{"Index", "Sample"+"("+TimeUnits+")"} 
		columnHdr = append(columnHdr, AddOn...)
	} else if rType == "Mean" {
		AddOn := []string{"Samples", "Mean"+"("+TimeUnits+")", "StdDev"} 
		columnHdr = append(columnHdr, AddOn...)
	} else if rType == "CI" {
		AddOn := []string{"Samples", "Mean"+"("+TimeUnits+")", "CI"} 
		columnHdr = append(columnHdr, AddOn...)
	} else if rType == "Range" {
		AddOn := []string{"Samples", "Min"+"("+TimeUnits+")", "Q1", "Q2", "Q3", "Max"+"("+TimeUnits+")"}
		columnHdr = append(columnHdr, AddOn...)
	} 

	// write it out
	f, err0 := os.OpenFile(csvFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err0 != nil {
		panic(err0)
	}


	headerLine := strings.Join(columnHdr, ",")+"\n"
	f.WriteString(headerLine)
	written := []string{headerLine}
	return written
}

// AddCSVData puts in one or more data rows to the csv file output
func (msrg *MsrGroup) AddCSVData(csvFileName, exprmntName string, data []string ) []string {

	written := []string{}

	// copy the fixedRow data into a new slice
	dataRow := make([]string, len(fixedRow))
	copy(dataRow, fixedRow)
	
	// add the user data that goes in starting at index 2
	for idx:=0; idx< len(data); idx++ {
		dataRow[idx+2] = data[idx]
	}

	// there should be a file here already
	f, err0 := os.OpenFile(csvFileName, os.O_APPEND|os.O_WRONLY, 0644)
	if err0 != nil {
		panic(err0)
	}

	// write in the measure name
	dataRow[1] = msrg.Measures[0].MsrName

	if rowType == "Samples" {
		// list the measured samples.  Add the index and value to the dataRow
		dataRow = append(dataRow,"")  // for the index
		dataRow = append(dataRow,"")  // for the value

		// visit every sample
		for idx:=0; idx<len(msrg.Measures); idx++ {

			dataRow[len(dataRow)-2] = strconv.Itoa(idx+1)
		
			// write in the value
			dataRow[len(dataRow)-1] = strconv.FormatFloat(msrg.Measures[idx].Value, 'g', 4, 64)	

			// make the row and write it in	
			csvRow := strings.Join(dataRow,",")+"\n"
			_, err0 := f.WriteString(csvRow)
			if err0 != nil {
				panic(err0)
			}
			written = append(written, csvRow)
		}
		f.Close()
		return written
	} else if rowType == "Mean" {

		// report mean and standard deviation of measurements in group
		samples, mean, stddev := msrg.MsrStats(false) 

		// convert numeric values into strings	
		stats := []string{strconv.Itoa(samples), strconv.FormatFloat(mean, 'g', 4, 64), 
			strconv.FormatFloat(stddev, 'g', 2, 64)}

		// add to the dataRow
		dataRow = append(dataRow, stats...)

	} else if rowType == "CI" {
		
		// report
		var ci float64
		samples, mean, stddev := msrg.MsrStats(true) 
		if samples > 1 {
			// formula for 95% confidence interval
			ci = 1.96*stddev/math.Sqrt(float64(samples-1))
		} 
		// convert numeric outputs into string
		stats := []string{strconv.Itoa(samples), strconv.FormatFloat(mean, 'g', 4, 64), strconv.FormatFloat(ci, 'g', 2, 64)}

		// extend dataRow with results
		dataRow = append(dataRow, stats...)

	} else if rowType == "Range" {

		// get min, max, quartiles
		samples, pstats := msrg.MsrRange()

		// include number of samples over which ranges are computed
		stats := []string{strconv.Itoa(samples)}

		// include every statistical result
		for idx:=0; idx<len(pstats); idx++ {
			stats = append(stats, strconv.FormatFloat(pstats[idx], 'g', 4, 64))
		}

		// extend the dataRow		
		dataRow = append(dataRow, stats...)
	}

	// make the row and write it in	
	csvRow := strings.Join(dataRow,",")+"\n"
	_, err0 = f.WriteString(csvRow)
	if err0 != nil {
		panic(err0)
	}
	written = append(written, csvRow)
	f.Close()
	return written
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

func ReturnParamsDict(exprFileName string, exprName string) map[string]string {
	if len(exprFileName) == 0 {
		rtn := make(map[string]string)
		return rtn
	}
	paramList, err := os.ReadFile(exprFileName)
	if err != nil {
		panic(err)
	}

	example := make([]map[string]string,1)
	example[0] = make(map[string]string)

    err = yaml.Unmarshal(paramList, &example)
	if err != nil {
		panic(err)
	}
	for _, paramDict := range example {
		if paramDict["name"] == exprName {
			rtnDict := make(map[string]string)
			for key, value := range paramDict {
				if key=="name" {
					continue
				}
				pieces := strings.Split(key,",")
				rtnDict[pieces[0]] = value	
			}
			return rtnDict
		}
	}
	emptyParams := make(map[string]string)
	return emptyParams
}


// SaveMeasureCSV constructs a csv representation of a measurement
func SaveMeasureCSV(dataFile string, hdr, values []string) []string {
	headerMap := ReturnParamsDict(ExprmntsFile, ExprmntName) 

	written := make([]string,0)

	columnNames := make([]string,0)
	for key := range headerMap {
		columnNames = append(columnNames, key)
	}
	sort.Strings(columnNames)

	// make a header if the csv file is empty
	_, err0 := os.Stat(dataFile)
	if err0 != nil {
		f, err0 := os.OpenFile(dataFile, os.O_CREATE|os.O_WRONLY, 0644)
		if err0 != nil {
			panic(err0)
		}

		hdr = append(hdr, columnNames...)

		headerLine := strings.Join(hdr, ",")+"\n"
		f.WriteString(headerLine)
		written = append(written, headerLine)
		f.Close()
	}

	for _, column := range columnNames {
		values = append(values, headerMap[column])
	}

	resultsCSV := strings.Join(values,",")+"\n"
	written = append(written, resultsCSV)
 
	// write to file
	f, err0 := os.OpenFile(dataFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err0 != nil {
		panic(err0)
	}
	f.WriteString(resultsCSV) 
	f.Close()
	return written
}
