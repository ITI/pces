// to validate your yaml or json file against this schema, use the following commands:

// validate cp file: 'cue vet -c --schema "#cp" schema.cue cp.yaml' (or cp.json)
// validate cpinit file: 'cue vet -c --schema "#cpinit" schema.cue cpinit.yaml' (or cpinit.json)
// validate map: 'cue vet -c --schema "#map" schema.cue map.yaml' (or map.json)
// validate exec: 'cue vet -c --schema "#exec" schema.cue funcExec.yaml' (or funcExec.json). todo: devExec
// validate exp: 'cue vet -c --schema "#exp" schema.cue exp.yaml' (or exp.json)
// todo : topo in progress

// -c ensure concrete values. see: https://cuelang.org/docs/references/spec/#:~:text=A%20value%20is%20concrete%20if,in%20the%20lattice%20is%20important.

// if running the command returns nothing, that means it succeeded, otherwise there would be error messages.
// to note, ex: 'cue eval -c --schema "#cp" schema.cue cp.yaml' also evaluate, validate, and prints out the config



//---------------------------data file---------------------------------------
#ExecType: "static" | "deterministic" | "random" |"stateful"
#apicpname: "client-server" | "simple-AES-chain" | "simple-det-test" | "simple-random-branch" | "simple-RSA-chain" | "simple-bit-test" | "encryptPerf"
#apicptypes: "RSAChain" | "SimpleWeb" | "AESChain" | "DeterministicTest" | "RandomBranch" | "StatefulTest" | "encryptPerf"
#apihostnames: "hostNetA1" | "hostNetB1" | "hostNetA2" | "hostNetT1" | "host-0" |"host-1" | "host-2" | "host-3" | "src" | "ssl"

#apifunctypes: {
    "client-server": "generate" | "serve"
    "simple-AES-chain": "generate" | "AES-encrypt" | "AES-decrypt" | "consume"
    "simple-det-test": "mark" | "testFlag" | "process"
    "simple-random-branch": "generate" | "branch" | "consume"
    "encryptPerf": "process" | "select" | "generate"
}

#apifunclabels: {
    "client-server": "client" | "server" | ""
    "simple-AES-chain": "src" | "encrypt" | "decrypt" | "consumer" | ""
    "simple-det-test": "src" | "select" | "consumer1" | "consumer2" | ""
    "simple-random-branch": "src1" | "src2" | "select" | "consumer1" | "consumer2" | ""
    "simple-RSA-chain": "src" | "decrypt" | "encrypt" | "sink" | "branch" | ""
    "simple-bit-test": "branch" | "consumer1" | "consumer2" | "src" | ""
    "encryptPerf": "host-0" | "host-1" | "host-2" | "host-3" | "crypto" | "src" | "data" | "initiate"
}

#apimsgtypes: {
    "client-server": "initiate" | "request" | "response"  | ""
    "simple-AES-chain": "initiate" | "data" | "encrypted" | "decrypted"  | "" | "consume"
    "simple-det-test": "initiate" | "marked" | "result"  | ""
    "simple-random-branch": "initiate" | "data" | "selected"  | ""
	"simple-RSA-chain": "encrypted" | "decrypted" | "data" | "initiate" | ""
	"simple-bit-test": "result" | "marked" | "" | "initiate"
    "encryptPerf": "data" | "initiate"
}

//---------------------------------entry point-----------------------------------------
// work in progress 
#Schema: #cp | #cpinit | #map | #exec | #exp

//----------------------------root--------------------------------------------


#cp: {
    prebuilt?: bool
    dictname: string
    patterns: {
        [string]: #Pattern
    }
}

#map: {
    dictname: string
    map: [string]: #Map
}

#cpinit: {
   prebuilt?: bool
   dictname: string
   initlist: #InitList
}

#exec: {
	listname: string
	times: [#apifunctypesFullList]: [...#Times &{functype: #apifunctypesFullList}]
    }


#exp: {
    expname: string
    parameters: [...#Parameter]
}

//----------------------------cp--------------------------------------------
#Pattern: {
    cptype: #apicptypes
    name: #apicpname
    funcs: [...#Func &{
        functype: #apifunctypes[name]
        label: #apifunclabels[name]
        exectype: #ExecType
    }] 
    edges: [...#Edge &{
        srclabel: #apifunclabels[name]
        dstlabel: #apifunclabels[name]
        msgtype: #apimsgtypes[name]
        edgelabel:#apifunclabels[name]
    }] 
}

#Func: {
    functype: string
    label: string
    exectype: string
}

#Edge: {
    srclabel: string
    dstlabel: string
    msgtype: string
    edgelabel: string
}


// -----------------------------cpInit------------------------------------


#InitList: {
    [string]: #CpInitPattern &{
        name: #apicpname
        cptype: #apicptypes
        params: 
        [string]: #Param &{
            patternname: #apicptypes
            label: #apifunclabels[name]
            response:[...#Response &{
                   inedge: #InEdge &{
                        srclabel: #apifunclabels[name]
                        msgtype: #apimsgtypes[name]
                }
                   outedge?: #OutEdge &{
                        dstlabel: #apifunclabels[name]
                        msgtype: #apimsgtypes[name]
                }
                  msgselect?: [...#OutEdge &{
                        dstlabel: #apifunclabels[name]
                        msgtype: #apimsgtypes[name]
                }]
            }]
        } 
         msgs: [...#Msg &{
            msgtype: #apimsgtypes[name]
        }] 
    } 
}


#CpInitPattern: {
    name: string
    cptype?: string
    useyaml: bool
    params: [string]: #Param
    exectype?: #Exectype
    msgs: [...{...}]
}

#Exectype: {
    [string]: #ExecType
}

#InEdge: {
    srclabel: string
    msgtype: string
}

#OutEdge: {
    dstlabel: string
    msgtype: string
}

#Param: {
    patternname: string
    label: string
    actions?: [...#Actions]
    state?: #State
    funcselect?: string
    response: [...#Response]
}

#newParam: {
    patternname: string
    label: string
    actions?: #Actions
    state?: #State
    funcselect?: string
    response: [...#Response]
}

#Actions: {
    prompt: #Prompt
    action: #Action
    limit: number
    choice: string
}

#Action: {
    select: string
    costtype: string
}

#Prompt: {
    srclabel: string
    msgtype: string
}

#State: {
    FuncSelect?: string
    testflag?: string
    failperiod?: string
    test?: string
    hosts?: string
}

#Response: {
    inedge: #InEdge
    outedge?:#OutEdge
    msgselect?: #OutEdge: float
    period: number & !=0
    choice?: string
}


#Msg: {
    msgtype: string 
    pcktlen: number
    msglen: number 
}


//-------------------------map----------------------------------------


#Map: {
    patternname:#apicpname
    funcmap: [#apifunclabels[patternname]]: #apihostnames
}

// -------------------------exp----------------------------------------



#paramObject: "Interface" | "Router" | "Network" | "Host" | "Switch" | "Filter"
#Attrbname: "media" | "*" | "name"
#Attrbvalue: "wired" | "wireless" | "" | "crypto"
#paramOptions: "latency" | "bandwidth" | "delay" | "CPU" | "trace" | "" | "MTU" | "capacity" | "model" | "buffer" | "accelerator"

#Parameter: {
    paramObj: #paramObject
    attributes: [...#Attribute & {
        attrbname: #Attrbname
        attrbvalue: #Attrbvalue
    }]
    param: #paramOptions
    value: string
}

#Attribute: {
    attrbname: string
    attrbvalue: string 
}

// --------------------------exec---------------------------------------
#apicpu: "x86" | "pentium" | "M2" | "M1" | "accel-x86"
#apifunctypesFullList: "generate" | "serve" | "AES-encrypt" | "AES-decrypt" | "consume" | "mark" | "testFlag" | "process" | "branch" | "RSA-decrypt" | "RSA-encrypt" | "pki-cache" | "pki-server" | "decrypt-aes" | "encrypt-aes" | "passThru" | "encrypt-rsa" | "decrypt-rsa" | "rtt"


#Times: {
        functype: string
        processortype: #apicpu
        pcktlen: int
        exectime: 0 | float
    }

// --------------------------topo---------------------------------------
// todo