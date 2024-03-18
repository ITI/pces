package main

// first step: use the following command to download cue: 
// 'go install cuelang.org/go/cmd/cue@v0.7.1'

// second step: to validate your yaml or json file against this schema, navigate to /input in the terminal and execute the following commands:

// to validate cp file: 'cue vet -c --schema "#cp" data_contraint.cue schema.cue cp.yaml' (or cp.json) - prints nothing if successful, else prints out the error messages
// or 'cue eval --schema "#cp" data_constraint.cue schema.cue cp.yaml' - this prints out the config if successful, else prints out the error messages

// only the --schema and yaml arguments change for each file you want to validate. --schema has the same name as the yaml file except for FuncExec(it is just #exec)
// to validate cpinit file: 'cue vet -c --schema "#cpinit" data_constraint.cue schema.cue cpinit.yaml' (or cpinit.json)
// to validate map: 'cue vet -c --schema "#map" data_constraint.cue schema.cue map.yaml' (or map.json)
// to validate exec: 'cue vet -c --schema "#exec" data_constraint.cue schema.cue funcExec.yaml' (or funcExec.json). todo: devExec
// or cue eval --schema "#exec" data_constraint.cue schema.cue funcExec.yaml (or funcExec.json)
// to validate exp: 'cue vet -c --schema "#exp" data_constraint.cue schema.cue exp.yaml' (or exp.json)
// to validate topo: 'cue vet -c --schema "#topo" data_constraint.cue schema.cue topo.yaml' (or topo.json)

// -c ensure concrete values. see: https://cuelang.org/docs/references/spec/#:~:text=A%20value%20is%20concrete%20if,in%20the%20lattice%20is%20important.

// documentation for cue: https://cuetorials.com


//---------------------------data file---------------------------------------

// moved to data_contraint.cue

//---------------------------------entry point-----------------------------------------
// work in progress 
#Schema: #cp | #cpinit | #map | #exec | #exp

//----------------------------root--------------------------------------------


#cp: {
    prebuilt?: bool
    dictname: string
    patterns!: {
        [string]: #Pattern
    }
}

#map: {
    dictname: string
    map!: [string]: #Map
}

#cpinit: {
   prebuilt?: bool
   dictname: string
   initlist!: #InitList
}

#exec: {
	listname: string
	times!: [#apifunctypesFullList]: [...#Times &{functype: #apifunctypesFullList}]
    }


#exp: {
    expname: string
    parameters!: [...#Parameter]
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
// issues: yaml files with | (string) is causing issues. Need to confirm whether we can remove this requirement.

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
    cptype: string
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

#value: {
    "CPU": string
    "model": string
    "accelerator": bool
    "bandwidth": float
    "buffer": float
    "capacity": float
    "latency": float
    "delay": float
    "MTU": int
    "trace": bool
}

#Parameter: {
    paramObj: #paramObject
    attributes: [...#Attribute & {
        attrbname: #Attrbname
        attrbvalue: string
    }]
    param: #paramOptions
    value: #value[param]
}

#Attribute: {
    attrbname: string
    attrbvalue: string 
}

// --------------------------exec---------------------------------------
#Times: {
        functype: string
        processortype: #apicpu
        pcktlen: int
        exectime: 0 | float
    }

// --------------------------topo---------------------------------------

#topo: {
    name: string
    networks: [...#Network]
    routers: [...#Router] 
    hosts: [...#Host]
    switches:[...#Switch] 
    filters: [...#Filter]
}

#Router: {
    rname: #apirouternames // could we change name to rname for namespace sake? makes validation easier. this would apply for the other definitions as well
    groups: [...]
    model: #apimodelnames
    interfaces: [...#Interface & {
        name: =~ "intrfc@\(rname)\\[\\d*\\.?\\d+\\](\\[\\d*\\.?\\d+\\])?" // do we need to do something like this?
        device: rname
    }]
}

#Filter: {
    name: #apifilternames
    groups: [...]
    cpu: #apicpu 
    model: #apimodelnames
    filtertype: #apifiltertype
    network: ""
    interfaces: [...#Interface]
}

#Host: {
    name: #apihostnames
	groups: [...]
	model: #apimodelnames
	cpu: #apicpunames
    interfaces: [...#Interface]
}

#Network: {
    name: #apinetnames
    groups: [...]
    netscale: string
    mediatype: #apimediatypes
    hosts: [...#apihostnames]
    routers: [...#apirouternames]
    filters: [...#apifilternames]
    switches: [...#apiswitchnames]
}

#Switch: {
    name: #apiswitchnames
    brdcstdmn: #apibrdnames
    model: #apimodelnames
}

#Interface: {
    name: #apiintrfcnames
    groups: [...]
    devtype:   #devtypes
    mediatype: #apimediatypes
    device:    #apidevnames
    cable:     string
    carry:     string
    wireless: [...]
    faces: #apinetnames
}