#tag MrNesbits Configuration
## Configuration Values for MrNesbits

MrNesbits supports configuring certain performance parameters of objects in the simulation, at run-time.  It does so through expression in a YAML file described below in [YAML Format](#YAML Format) Before going into details it is worthwhile to understand the overall structure.

MrNesbits can configure six types of network objects: Switch, Router, Host, Network, Interface, and Filter. ‘Interface’ refers to a network interface device, and ‘Filter’ refers to a device through which traffic passes and may be modified, e.g., filtered with access control rules, have Network Address Translation performed, do encryption/decryption.  Every device has a unique name (ascii string), and may be a member of one or more ‘groups’.    A group is just a label we can use when selecting components to configure.

Every simulator object has a number of *attributes*.  The first and most obvious attribute is that of *type*,  e.g., Host, Router, etc.  An object’s unique name is an attribute, and every group it belongs to is an attribute.  Indeed, these are the only kinds of attributes we ascribe to Hosts, Filters, and Interfaces.   Switches and Routers have an additional one of ‘model’, a string that can identify the manufacturer and model number of the device.  Networks have the additional attributes of ‘media’, meaning the communication media (e.g. wired, wireless, satellite) and size (of the IP address space it spans).

A MrNesbits configuration file is a collection of configuration *statements.*  Every statement identifies the type of the object(s) being configured and a list of its attributes values that must all be satisfied for the object to take the configuration value.   This format gives us great flexibility.   We can identify just one object by specifying its type and its name, and we can identify **all** objects of a given type by listing the attribute to be a wildcard.  We can assign a common group identity to all interfaces in a router and with one statement configure them all, and we can identify the set of all routers that share a common model and have the configuration apply to each of these.  

The columns in the table below identify potential attributes, the rows simulation object types, and a checkmark means that the selected type of object has the selected type of attribute.

| type/attribute | name | group | CPU | model | media | scale |
|----------------|:----:|:-----:|:---:|:-----:|:-----:|:-----:|
| Switch         | x    | x     |     | x     |       |       |
| Router         | x    | x     |     | x     |       |       |
| Host           | x    | x     | x   | x     |       |       |
| Filter         | x    | x     | x   | x     |       |       |
| Interface      | x    | x     |     |       | x     |       |
| Network        | x    | x     |     |       | x     | x     |
			Table 1: Network object attributes by network object type

The types of configuration parameters that can be applied are described below.
* CPU (string).   Processor type.
* Model (string). Model identifier of device.
* Accelerator (bool).  Whether device has cryptographic accelerator.
* Bandwidth (float, Mbits/sec).  Traffic throughput.
* Buffer size (float, Mbytes)
* Capacity (float, Mbits/sec).   Total traffic throughput supported by a network.
* Latency (float, secs). Average time for the first bit of a packet to traverse a network.   
* Delay (float, secs). Time for 1st bit of packet to transit an interface.
* MTU (int).  Maximum transmission unit.
* trace (bool).   Whether output trace should include events at device.

The columns of the table below identity device type, rows identify configuration parameters, and a checkmark indicates that the selected type of device can have the selected type of configuration parameter applied to it.

| parameter/type | Switch | Router | Host | Filter | Interface | Network |
|----------------|:------:|:------:|:----:|:------:|:---------:|:-------:|
| CPU            |        |        | x    | x      |           |         |
| model          | x      | x      |      | x      |           |         |
| accelerator    |        |        | x    | x      |           |         |
| bandwidth      |        |        |      |        | x         | x       |
| buffer         | x      | x      |      |        |           |         |
| capacity       |        |        |      |        |           | x       |
| latency        |        |        |      |        | x         |         |
| delay          |        |        |      |        | x         |         |
| MTU            |        |        |      |        | x         |         |
| trace          | x      | x      | x    | x      | x         | x       |
				Table 2: Network object parameters by network object type
### YAML Configuration

A yaml configuration file is basically comprised of a list (not necessarily ordered in any particular way) of *configuration statements*.   The configuration statement identifies the type of network objects that may be impacted by the statement, a subset of objects of that type which are to have some parameter set, which parameter, and the value to which it is set.   Specification of parameter and parameter setting is obvious, specification of objects to receive that setting somewhat less so.

One can easily specify that *all* objects of the named type are to have a parameter set, and one can easily specify that an object with a particular specified name is to have a parameter set.  We support specification with greater refinement than these two extremes, using the attributes described earlier in Table 1.   Given an object we can always answer the question ‘Does *this* attribute of *this* object have *this* value” and use the question as a condition which must be met for an object to be modified by the new parameter setting.  In fact what we do is include in the configuration statement a list of these conditions, all of which must be met by an object for the parameter setting to apply.   The extreme of a wildcard is included by specifying a list with one member that indicates the wildcard.  The extreme of applying the parameter setting to exactly one object with the specified name is included by specifying a list with one member that identifies that the ‘name’ attribute must have the specified value.

This we see that the configuration statement is comprised of four pieces of information.  
* key `paramObj` identifies the type of network object that can be affected by the statement.  Permissible values are {Switch, Router, Host, Filter, Interface, Network}.
* key `attributes`.  The value is a list of *attribute identifiers*.  Each such identifier is a structure with two fields, ‘attrbname’ and ‘attrbvalue’.   The former takes values of either ‘*’ (the wildcard), or one of the column headers of Table 1.  The latter takes string-encoded values quantifications of the attribute’s name, e.g., a group name or label for a particular type of CPU.   The list of attributes identifies conditions to be met by a network object in order for for it to use the configuration parameter described in this configuration statement.
* key `param`.  The value identifies the particular parameter of the network object whose value is to be set.  This key is one of the row labels in Table 2.
* key `value`.  The value describes what should be applied to the identified parameter.

An example configuration file is listed below in yaml to illustrate these points.  The key `expname` is just a label for the configuration.  Key `parameters` has as its value a list of configuration statements, with the format described earlier.

```
expname: BITS-1
parameters:
    - paramObj: Interface
      attributes:
        - attrbname: media
          attrbvalue: wired
      param: delay
      value: "10e-6"
    - paramObj: Network
      attributes:
        - attrbname: media
          attrbvalue: wired
      param: latency
      value: "10e-3"
    - paramObj: Interface
      attributes:
        - attrbname: media
          attrbvalue: wireless
      param: latency
      value: "10e-4"
    - paramObj: Network
      attributes:
        - attrbname: media
          attrbvalue: wireless
      param: capacity
      value: "1000"
    - paramObj: Interface
      attributes:
        - attrbname: '*' 
          attrbvalue: ""
      param: bandwidth
      value: "100"
    - paramObj: Interface
      attributes:
        - attrbname: '*' 
          attrbvalue: ""
      param: MTU 
      value: "1500"
    - paramObj: Host
      attributes:
        - attrbname: '*' 
          attrbvalue: ""
      param: CPU 
      value: x86 
    - paramObj: Filter
      attributes:
        - attrbname: name
          attrbvalue: crypto
      param: CPU
      value: accel-x86
    - paramObj: Filter
      attributes:
        - attrbname: name
          attrbvalue: crypto
      param: accelerator
      value: "true"
    - paramObj: Host
      attributes:
        - attrbname: '*'
          attrbvalue: ""
      param: trace
      value: "true"
    - paramObj: Switch
      attributes:
        - attrbname: '*'
          attrbvalue: ""
      param: model
      value: Slow
    - paramObj: Switch
      attributes:
        - attrbname: '*'
          attrbvalue: ""
      param: trace
      value: "true"
    - paramObj: Router
      attributes:
        - attrbname: '*'
          attrbvalue: ""
      param: trace
      value: "true"
    - paramObj: Switch
      attributes:
        - attrbname: '*'
          attrbvalue: ""
      param: buffer
      value: "100"
    - paramObj: Router
      attributes:
        - attrbname: '*'
          attrbvalue: ""
      param: buffer
      value: "256"
    - paramObj: Router
      attributes:
        - attrbname: '*'
          attrbvalue: ""
      param: model
      value: Slow

```

### Statement Ordering
It is possible for an object to satisfy the conditions of  multiple configuration statements all identify the same parameter to set, but differ in their settings.    We do not require that configuration statements appear in any kind of sorted order in lists like that above, but due process this list to achieve an intuitive priority ordering.   If statement S1 and S2 both set parameter P and if object O satisfies all the attribute identifiers of both S1 and S2, then it must be the case that the set of attribute identifiers of S1 are a strict subset of those in S2, or vice-versa.    O satisfies both conditions, but one of these is more stringent than the other (the one with more attribute identifiers).    We give the more stringent statement priority.   This allows us to do things like set the bandwidths of all router interfaces to 1000Mbits, except for the interfaces attached to routers on WLAN scale networks, which get 5000Mbits

```
  - paramObj: Interface
	attributes:
		- attrbName: '*'
		  attrbValue: ""
		- attrbName: group
		  attrbValue: "router"
	param: bandwidth
	value: "1000"
	
  - paramObj: Interface
	attributes:
		- attrbName: group
		  attrbValue: "WLAN"
		- attrbName: group
		  attrbValue: "router"
	param: bandwidth
	value: "5000"

```

These statements rely on having all interfaces attached to routers be members of a user-defined group called ‘routers’, and all devices facing a WLAN scale network be members of a user-defined group called ‘WLAN’.  In the above the ordering of the two configuration statements does not matter.