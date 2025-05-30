package mrnes

// the data objects and functions contained within this file (chan.go)
// support the use of a mrnes defined chanGuard struct. This construct
// defines two golang channels that together are used to govern the request
// and provisioning 
import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"log"
	"os/exec"
	"os"
	_ "io"
	"time"
	"net"
)

// sleepMilliSecs encodes the number of milliseconds we cause
// a goroutine's function to pause when it is awaiting activity on a channel (for read, or for
// write) and we will not release the function until either the expected activity
// happens, or (through other means) we determine that we need to release the function
var sleepMilliSecs time.Duration = 100

// cmdTrace allows logging statements throughout the functions in chan.go
var cmdTrace bool = true


// chanCmd types different codes for messages passed across
// a chanGuard's cmd channel
type chanCmd int
const (
	none chanCmd = iota  // in case the existence of a message and not any othe meaning is needed
	start     // signal from consumer to producer to start the packet acquistion process.
              // depending on the configuration this may be needed for each packet, or needed just once
	terminate // fold up tent and go home
)

// chanCmdStr used in logging statements
var chanCmdStr map[chanCmd]string = map[chanCmd]string{
	none: "None", start:"start", terminate: "terminate"}

// a producer and consumer engage in a request-response-ack protocol, putting
// both in different states that matter when deciding what and if to poll on the
// communication channel.  The states for the producer are captured in the
// producerState type
type producerState int
const (
	ps_waiting  producerState = iota  // producer is waiting for further directive
	ps_pulling						  // producer pulling at the source
	ps_terminated                     // a termination command has been received or the
									  // end of channel detected.  No further packet acquisition
)

// producerStateStr used in logging statements
var producerStateStr map[producerState]string = map[producerState]string{
	ps_waiting: "waiting", ps_pulling: "pulling", ps_terminated: "terminated"}

// the consumerState reflects the consumer's current relationship with the chanGuard and its information flows
type consumerState int
const (
	cs_entry     consumerState = iota // just built, nothing has happened yet
	cs_off                            // consumer is in a period when no packet has been asked for, nor expected
	cs_waiting                        // the consumer has requested (explicitly or implicitly) a packet, and awaits it
	cs_completed                      // a cycle of request-received has been completed
	cs_closed                         // transfers are over
)

var consumerStateStr map[consumerState]string = map[consumerState]string{
	cs_entry: "entry", cs_off: "off", cs_waiting: "waiting", cs_completed: "completed", cs_closed: "closed"}


// chanDataType encodes the type of data passing from the producer to
// consumer in a given chanGuard data channel.  Set once for the chanGuard
// at creation, used just to enable decoding on receipt, as the channel itself has state 'any'
//
type chanDataType int
const (
	gpckt       chanDataType = iota // gopacket.Packet
	int_value                       // int
	float_value                     // float64
	bool_value                      // bool
)
// use a chanGuard to control a channel where one goroutine
// pushes the address of a gopacket through through a channel to
// another. The assumption is that "consumer" create the goroutine
// to a "producer" that passes back the gopacket pointer through the
// 'pckt' channel.  Communication
// about the state of that relationship, i.e., the consumer telling the
// producer to request, the consumer telling the producer to finish
//	The owner name is included just for tracing, plays no role in the logic
type chanGuard struct {
	data      chan any		// where the data passes
	cmd       chan chanCmd	// carries commands from consumer to producer
	dataType  chanDataType  // type of the data carried, used for decoding
	ownerName string		// included to add information in logging statements
}

// pcacpSrcType indicates whether the source of packets is ultimately from
// a live interface or a file of pcap captures.  Most everything we do
// does not matte on the type of the source, but occasionally it does
// so we encode the type for readability
//
type pcapSrcType int
const (
	offlineType pcapSrcType = iota
	onlineType
)

// reportTransition creates and logs information about execution,
// typically when the cmdTrace flag is set.
func reportTransition(objName, funcName, msg string) {
	fmt.Sprintf("%s in %s %s\n", objName, funcName, msg)
	if logging {
		log.Printf("%s in %s %s\n", objName, funcName, msg)
	}
}

// ---------- Functions called by the producer ---------
// expectCmds blocks the caller until something shows up
// on the cmd channel, and it returns true if the type of command
// received is what is expected 
//
func (cg *chanGuard) expectCmd(expected chanCmd) bool {

	if cmdTrace {
		msg := fmt.Sprintf("expects command %s", chanCmdStr[expected])
		reportTransition(cg.ownerName, "expectCmd", msg)
	}

	// the only way out of this loop is through a return
	for {
		select {
			// the required command
			case cmd := <-cg.cmd: 
				// does it match what is expected?
				if cmd != expected {
					fmt.Printf("unexpected cmd return %v != %v\n", cmd, expected)
				}
				return cmd==expected
			default: 
				// take a nap and try again
				time.Sleep(sleepMilliSecs * time.Millisecond)
		}
	}
	// compiler doesn't do deep enough analysis to see
	// that we don't ever reach this statement
	return false
}

// reportPckt is used by a producer to push back to a consumer something read off
// the data source.  A boolean is returned indicating whether the channel got
// a control message from the consumer before that data was accepted
func (cg *chanGuard) reportPckt(pckt any) bool {

	// try first to actually push the dataGram
	if cmdTrace {
		reportTransition(cg.ownerName, "reportPckt", "entry")
	}

	// the only way to escape this loop is through a return statement
	for {
		select {
		case cg.data <- pckt:
			if cmdTrace {
				reportTransition(cg.ownerName, "reportPckt", "successful pckt report")
			}
			return true
		case cmd := <-cg.cmd:
			// we see a command from the consumer before completing the write on the data channel
			if cmdTrace {
				msg := fmt.Sprintf("sees %s on command channel", chanCmdStr[cmd])
				reportTransition(cg.ownerName, "reportPcket", msg)
			}
			return false

		default:
			// nothing yet, so let other things happen
			time.Sleep(sleepMilliSecs * time.Millisecond)
		}
	}
	// statement below is never reached, but the compiler needs it
	return false
}


// ---- functions called by the consumer -----

// createChanGuard creates a new chanGuard object and makes its two channels.
// Both of these have buffer length of 1
//
func createChanGuard(ownerName string, dataType chanDataType) *chanGuard {
	cg := new(chanGuard)

	// we need a buffered channel to permit asynchronous sends and receives on the channel.
	// otherwise one or the other of the endpoints must be present and blocked on the channel
	// for the operation to be completed by the other on access to the channel.
	//
	cg.dataType = dataType
	cg.data = make(chan any,1)
	cg.cmd = make(chan chanCmd, 1)
	cg.ownerName = ownerName
	return cg
}
// pushCmd takes the chanCmd argument and does not
// return to caller until its push on that channel
// is selected.  A short snooze happens if we cannot
// immediate push it through
func (cg *chanGuard) pushCmd(cmd chanCmd) {

	if cmdTrace {
		msg := fmt.Sprintf("pushes command %s", chanCmdStr[cmd])
		reportTransition(cg.ownerName, "pushCmd", msg)
	}

	// the only escape from this loop is through a return
	for {
		select {
		case cg.cmd <- cmd:
			// the write to channel completed
			if cmdTrace {
				msg := fmt.Sprintf("accepts command %s", chanCmdStr[cmd])
				reportTransition(cg.ownerName, "pushCmd", msg)
			}
			return
		default:
			// wait a bit and try again 
			time.Sleep(sleepMilliSecs * time.Millisecond)
		}
	}
}

// acquirePckt is used by the consumer to see if there is a message on the data channel.
// If there is (possibly nil) return it and indicate whether whether the response
// represents the actual acquisition of a packet
//
func (cg *chanGuard) acquirePckt(timeout float64) (any, bool) {
	
	var timedOut bool = false
	var testTimeout bool = timeout>0.0

	var timeoutInNanoSecs = int64(timeout*1e9)

	if cmdTrace {
		reportTransition(cg.ownerName, "acquirePckt", "enters")
	}
	start := time.Now()
	for {
		select {
		case pckt := <-cg.data:
			fmt.Printf("%s acquires packet %v\n", cg.ownerName, pckt)
			return pckt, true 
		default :
			finish :=  time.Now()
			timedOut = time.Duration(timeoutInNanoSecs) < finish.Sub(start)
			if !testTimeout || !timedOut {
				time.Sleep(100*time.Millisecond)
			} else {
				return nil, false
			}
		}
	}
	return nil, false
}

// nxtPckt pulls the next packet from the named PacketSource.
// Returns a packet (if present, nil otherwise), and a chanCmd,
// which if received interrupts the wait for a packet
//
var pcktTries int = 0
func nxtPckt(ps *gopacket.PacketSource, cg *chanGuard) (gopacket.Packet, error, chanCmd) {
	for {
		select {
		case cmd := <-cg.cmd :
				return nil, nil, cmd
		default:
			pckt, err := ps.NextPacket()

			if pcktTries%100 == 0 {
				fmt.Printf("nxtPckt pass %d\n", pcktTries)
			}

			pcktTries += 1
			// here's the packet
			if pckt != nil && err == nil {
				pcktTries = 0
				return pckt, nil, none
			}

			// return when we see any error 
			if err != nil {
				return nil, err, none
			}
			// time.Sleep(time.Millisecond)
		}
	}

	// never reached but the compiler complains otherwise
	return nil, nil, none
}

// recvPcktsFromSrc receives pcap packets from a source
// (either online or offline) so long as they are available
// from the source, and the consumer has not signaled they are no
// longer required
// srcType is 'offlineType' or 'onlineType', depending
func recvPcktsFromSrc(pcnd *pcapNode) {

	var useScript bool = false		// flags whether the live source is started 
									// from a shell script we cause to execute
	var who string					// identifier used when logging traces
	var err error
	var goPckt gopacket.Packet
	var timedOut bool = false

	var cg *chanGuard = pcnd.cg

	// initialization depends on the source 
	switch pcnd.online {
		case true:

			// variables themselves used for either type of source, we know here to get them from onlineNode
			who = cg.ownerName		// used in logging reports

			// pcnd.script will be non-empty when at configuration we are told that this source
			// is brought to existence through the execution of a shell script.  In that case
			// we use the exec module to set up the execution and setup pipes for stdin 
			// for communicating with the process being executed.  Typically (but not required)
			// the script brings up a bash shell that will need a command feed to it to generate the traffic source)
			//
			if pcnd.script != "" {
				fmt.Printf("initializing with script %s\n", pcnd.script)
				// create the data structure describing this as-yet-unstarted execution
				// the assumption is that pcnd.script is a vetted path name for
				// a script that can be executed by call (e.g. has #!/usr/bin sh as the first line
				// and has execution permission)
				//
				cmd := exec.Command(pcnd.script)		

				// set up the pipe for stdin
				var in_err error
				pcnd.stdin, in_err = cmd.StdinPipe()
				if in_err != nil {
					fmt.Printf("In %s fatal error setting up stdin pipe", who)
					log.Printf("In %s fatal error setting up stdin pipe", who)
					os.Exit(1)
				}	
				if pcnd.stdin == nil {
					fmt.Printf("In %s fatal error stdin pipe is nil", who)
					log.Printf("In %s fatal error stdin pipe is nil", who)
					os.Exit(1)
				}

				// execute the script
				srt_err := cmd.Start()

				// look at the error messages coming back
				if srt_err != nil {
					log.Printf("In %s cmd.Start returns err %s\n", who, srt_err)
					log.Fatal(srt_err)
				} else if cmdTrace {
					log.Printf("In %s cmd.Start successfully executes\n", who )

				}

				// for now, wait a second
				time.Sleep(1*time.Second)
				useScript = true
			}

			// open the connection to the (possibly new) device,
			// if we used a script to create it we wait for no more
			// than pcnd.srcBuildTimeout seconds
			// timeout interface is in seconds, gap in times is in nanoseconds.
			// If we aren't using a script, an error is fatal
			//
			timeoutInNanoSecs := int64(pcnd.srcBuildTimeout*1e+9)

			pcnd.slowStart = time.Now()

			// if we aren't waiting for the device to come up we'll
			// exit the loop after 1 iteration.  Otherwise we exit
			// either when the device is recognized or we time out
			//
			for pcnd.handle == nil && !timedOut {
				//handle, err := pcap.OpenLive(pcnd.srcName, 65535, true, pcap.BlockForever)
				pcnd.handle, err = pcap.OpenLive(pcnd.srcName, 65535, true, time.Microsecond*2)
				// handle, err := pcap.OpenLive(pcnd.srcName, 65535, true, time.Microsecond*1)
				if err != nil || pcnd.handle == nil {
					// open error.  If we are waiting for the device to appear we expect this
					// and back off.  We are aren't then this is a fatal error
					//
					if !useScript {
						fmt.Printf("error %s on OpenLive(%s)\n", err, pcnd.srcName)
						log.Printf("error %s on OpenLive(%s)\n", err, pcnd.srcName)
						os.Exit(1)
					} else {
						// back off for a little time
						finish :=  time.Now()
						timedOut = time.Duration(timeoutInNanoSecs) < finish.Sub(pcnd.slowStart)
						if !timedOut {
							time.Sleep(100*time.Millisecond)
						}
					}
					continue
				}
			}

			// if it is still the case that the pcap handle is nil, that means
			// that we tried to use a script to create the virtual interface
			// but after repeated attempts that led to a time-out, we did not,
			// which constitutes an error
			//
			if pcnd.handle == nil {
				fmt.Printf("error on OpenLive(%s)\n", pcnd.srcName)
				log.Printf("error on OpenLive(%s)\n", pcnd.srcName)
				os.Exit(1)
			}

			// the virtual interface is established and some MAC address was given to it.
			// Save that for ARP stuff
			//
			virtIntrfc, _ := net.InterfaceByName(pcnd.srcName)
			pcnd.hwAddr = virtIntrfc.HardwareAddr
			fmt.Printf("virtIntrfcAddr is %v\n", pcnd.hwAddr)
		case false :
			pcnd.handle, err = pcap.OpenOffline(pcnd.srcName) 
			if err != nil {
				fmt.Printf("error on OpenOffline(%s)\n", pcnd.srcName)
				log.Printf("error on OpenOffline(%s)\n", pcnd.srcName)
				os.Exit(1)
			}
	}

	// source type dependency stuff mostly finished now

	// by default we are NOT doing lazy decoding and ARE doing copy decoding
	// so the packets coming from src will be concurrency safe
	src := gopacket.NewPacketSource(pcnd.handle, layers.LayerTypeEthernet)
	// src := gopacket.NewPacketSource(pcnd.handle, pcnd.handle.LinkType())

	// pcnd.pcktSrc = gopacket.NewPacketSource(pcnd.handle, pcnd.handle.LinkType())

	// no matter whether requestDriven is set or not, we require a 'start'
	// command to begin the pulling and so set the producer state to ps_waiting
	var state producerState = ps_waiting

	var in chan gopacket.Packet

	inSrc := src.Packets()
	if pcnd.online {
		in = inSrc
		fmt.Printf("channel to device established, capacity %d, enqueued %d\n", cap(in), len(in))
	} else {

		// the packets from a file will always be available.  To better 
		// create behaviour similar to when the interface is live, we
		// artificially include a real-time 1 second delay between successive
		// reports of packets
		//
		in = make(chan gopacket.Packet)
		go func(chOut chan gopacket.Packet, inSrc chan gopacket.Packet) {
			for {
				time.Sleep(2*time.Second)
				pckt := <- inSrc
				chOut <- pckt
			}
		}(in, inSrc)
	}
	
	// we stay in this loop until either the end of source is detected, we get
	// a termination command from the consumer, or we get an otherwise unexpected command
	// from the consumer
	//
	pcnd.slowStart = time.Now()
	timedOut = false
	timeoutInNanoSecs := int64(pcnd.slowTimeout*1e+9)

	for state != ps_terminated  && !timedOut {

		// we're waiting and there is a command but it isn't start.
		// That's taken to mean it is time to exit the loop
		//
		if state  == ps_waiting && !cg.expectCmd(start) {
			// anything other than 'start' implies termination
			state = ps_terminated
			break
		}

		// either we were waiting and got the start, 
		// or were already pulling. Set the state and start looking
		// for packets
		//
		state = ps_pulling
		var open bool
		select {
			case goPckt, open = <- in:

				fmt.Printf("%t %v\n", open, goPckt)

				// if goPckt is nil and closed is true, try once more
				// (because the bool is actually an indicator of whether 
				// the value was given after the channel was closed
				if goPckt == nil {
					goPckt, open = <- in
				}
				if !open {
					state = ps_terminated
					break
				}

				// otherwise, given a nil packet we'll take it as a timeout,
				// and in the case of online take another lap around the track
				if goPckt == nil && pcnd.online {
					continue
				}

				// go ahead and post the packet
				if goPckt != nil {
					fmt.Printf("packet delivered %v\n", goPckt)
				}
				if goPckt != nil && !cg.reportPckt(goPckt) {
					// negative means that before the packet was accepted by the data channel
					// we got a command on the command channel, which we interpret to mean
					// to terminate the source and don't try to push the packet
					//	
					if cmdTrace {
						msg := fmt.Sprintf("%s sees early termination cmd from phh=reportPckt", who)
						reportTransition(cg.ownerName, "recvPcktsFromSrc", msg)
					}
					state = ps_terminated
					break
				}
				// packet was successfully reported.
				// Whether we transition into ps_waiting from ps_pulling
				// remain in pulling depends on configuration.  If anotherr
				// request is not needed we can leave state in the state of ps_pulling
				if pcnd.requestDriven {
					// need to wait for another ask
					state = ps_waiting
				}
				pcnd.slowStart = time.Now()

			case _ = <- cg.cmd:
				// means we've been told to terminate before we acquired another
				// packet. Punch out.
				state = ps_terminated
				break

			default :
				// see if we timed out (and so close the connection),
				// or take a nap
				finish :=  time.Now()
				timedOut = pcnd.slowTimeout > 0 && time.Duration(timeoutInNanoSecs) < finish.Sub(pcnd.slowStart)
				if timedOut {
					fmt.Println("recvPckts timed out")
					state = ps_terminated
					break
				}
				time.Sleep(time.Millisecond)
		}

		/*
		// goPckt, pckt_err, _ := nxtPckt(pcnd.pcktSrc, pcnd.cg)

		// if there is no error and no command we try to report the packet,
		// but if we get a cmd instead we terminate
		if goPckt != nil && !cg.reportPckt(goPckt) {
			// negative means that before the packet was accepted by the data channel
			// we got a command on the command channel, which we interpret to mean
			// we terminate the source
			//	
			if cmdTrace {
				msg := fmt.Sprintf("%s sees early termination cmd from phh=reportPckt", who)
				reportTransition(cg.ownerName, "recvPcktsFromSrc", msg)
			}
			state = ps_terminated
			break
		}

		// if goPckt == nil and pckt_err == io.EOF we are done with the channel
		if goPckt == nil && pckt_err == io.EOF {
			fmt.Println("recvPckts receives EOF error from nxtPckt")
			state = ps_terminated
			break
		}

		if goPckt == nil && os.IsTimeout(pckt_err) {
			fmt.Println("recognized timeout in recvPckts")
			time.Sleep(time.Millisecond)
			continue
		}

		// if goPckt != nil we delivered it and can go back for another
		if goPckt != nil && pckt_err == nil {
			// reset the packet acquisition time timer.
			pcnd.slowStart = time.Now()

			// Whether we transition into ps_waiting from ps_pulling
			// remain in pulling depends on configuration.  If anotherr
			// request is not needed we can leave state in the state of ps_pulling
			if pcnd.requestDriven {
				// need to wait for another ask
				state = ps_waiting
			}
			continue
		}

		// goPckt is nil and pckt_cmd is none, which
		// we interpret as a timeout.  We check and see if this loop has
		// timed out
		finish :=  time.Now()
		timedOut = pcnd.slowTimeout > 0 && time.Duration(timeoutInNanoSecs) < finish.Sub(pcnd.slowStart)
		if !timedOut {
			time.Sleep(sleepMilliSecs*time.Millisecond)
		}
	*/
	}
	// the key cleanup needed is to remove the interfaces that were created
	// and we should be able to do this executing a script
	return
}
