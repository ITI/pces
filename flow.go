package pces

import (
	"github.com/iti/mrnes"
	"github.com/iti/evt/evtm"
	"github.com/iti/evt/vrtime"
)

var MrnesFlowsCnt int = 0
func NxtMrnesFlowID () int {
	MrnesFlowsCnt += 1
	return MrnesFlowsCnt
}

type BckgrndFlow struct {
	ExecID int
	MajorID int
	ConnectID int
	FlowType mrnes.ConnType
	Src string
	Dst string
	RequestedRate float64
	AcceptedRate  float64
	RtnDesc mrnes.RtnDesc
}

var BckgrndFlowList map[int]*BckgrndFlow = make(map[int]*BckgrndFlow)

func CreateBckgrndFlow(evtMgr *evtm.EventManager, srcDev string, dstDev string, 
		requestRate float64, reservation bool, execID int, context any, hdlr evtm.EventHandlerFunction) (*BckgrndFlow, bool) {

	bgf := new(BckgrndFlow)
	bgf.RequestedRate = requestRate
	bgf.Src = srcDev
	bgf.Dst = dstDev
	bgf.MajorID = NxtMrnesFlowID()
	bgf.ExecID = execID
	bgf.FlowType = mrnes.MajorFlowConn
	bgf.RtnDesc.Cxt = context
	bgf.RtnDesc.EvtHdlr = hdlr
	bgf.ConnectID = 0			// indicating absence

	msg := CreateCmpPtnMsg()
	msg.MrnesConnDesc = mrnes.ConnDesc{Type: mrnes.MajorFlowConn, Latency: mrnes.Zero, Action: mrnes.Srt}

	msg.MrnesIDs.ExecID = execID
	msg.MrnesIDs.MajorID = bgf.MajorID
	msg.MrnesIDs.MinorID = 0
	msg.Rate = requestRate

	msg.MrnesRprts.Rtn.Cxt = bgf
	msg.MrnesRprts.Rtn.EvtHdlr = AcceptedBckgrndFlowRate  

	if reservation {
		msg.MrnesConnDesc.Type = mrnes.Reservation
		bgf.FlowType = mrnes.Reservation
	}

	var OK bool
	var acceptedRate float64
	bgf.ConnectID, acceptedRate, OK = netportal.EnterNetwork(evtMgr, srcDev, dstDev, 1500,
		&msg.MrnesConnDesc, msg.MrnesIDs, msg.MrnesRprts, requestRate, msg)

	if !OK {
		return nil, false
	}
	if bgf.FlowType == mrnes.Reservation {
		netportal.Rsrvd[bgf.MajorID] = acceptedRate
	}
	BckgrndFlowList[bgf.MajorID] = bgf

	return bgf, true	
}

func (bgf *BckgrndFlow) RmBckgrndFlow(evtMgr *evtm.EventManager, context any, hdlr evtm.EventHandlerFunction) {
	bgf.RtnDesc.Cxt = context
	bgf.RtnDesc.EvtHdlr = hdlr
	msg := CreateCmpPtnMsg()
	msg.MrnesConnDesc.Type = bgf.FlowType
	msg.MrnesConnDesc.Latency = mrnes.Zero
	msg.MrnesConnDesc.Action = mrnes.End

	msg.MrnesIDs.ExecID = bgf.ExecID
	msg.MrnesIDs.MajorID = bgf.MajorID
	msg.MrnesIDs.MinorID = 0
	msg.Rate = 0.0

	// should someone get reported to?
	msg.MrnesRprts.Rtn = new(mrnes.RtnDesc)
	msg.MrnesRprts.Rtn.Cxt = bgf
	msg.MrnesRprts.Rtn.EvtHdlr = BckgrndFlowRemoved	
	
	netportal.EnterNetwork(evtMgr, bgf.Src, bgf.Dst, 1500,
		&msg.MrnesConnDesc, msg.MrnesIDs, msg.MrnesRprts, 0.0, msg)

	if bgf.FlowType == mrnes.Reservation {
		delete(netportal.Rsrvd, bgf.MajorID)
	}
}

func BckgrndFlowRateChange(evtMgr *evtm.EventManager, cxt any, data any) any {
	bgf := cxt.(*BckgrndFlow)
	rprt := data.(*mrnes.RtnMsgStruct)
	bgf.AcceptedRate = rprt.Rate
	return nil
}
		 
func BckgrndFlowRemoved(evtMgr *evtm.EventManager, cxt any, data any) any {
	bgf := cxt.(*BckgrndFlow)
	delete(BckgrndFlowList, bgf.MajorID)	
	evtMgr.Schedule(bgf.RtnDesc.Cxt, data, bgf.RtnDesc.EvtHdlr, vrtime.SecondsToTime(0.0))

	return nil
}
		 
func AcceptedBckgrndFlowRate(evtMgr *evtm.EventManager, context any, data any) any {
	bgf := context.(*BckgrndFlow)
	rprt := data.(*mrnes.RtnMsgStruct)
	bgf.AcceptedRate = rprt.Rate
	evtMgr.Schedule(bgf.RtnDesc.Cxt, data, bgf.RtnDesc.EvtHdlr, vrtime.SecondsToTime(0.0))
	return nil
}

