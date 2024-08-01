package pces

// file desc-crypto.go holds structs, data structures, and methods used to
// build and read summaries of the measurements of cryptographic algorithms
// available to the pces model.  In particular the data structures
// can be accessed by a GUI used to guide model development

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// AlgSpec provides representation of crypto algorithm and key lengths
// for which we have timing measurements.  Used in the CryptoDesc struct
// below, which can be read by a GUI to align with the crypto algorithms
// and key lengths that are available
type AlgSpec struct {
	Name   string // e.g., 'aes','des'
	KeyLen []int  // lengths of keys used in measurements represented in the functional timings
}

// CryptoDesc is a structure used so that the GUI and simulator can
// 'see' the same set of crypto algorithms that might be chosen,
// and have their performance included in the system behavior
type CryptoDesc struct {
	Algs []AlgSpec
}

// createCryptoDesc is a constructor
func createCryptoDesc() *CryptoDesc {
	cd := new(CryptoDesc)
	cd.Algs = []AlgSpec{}
	return cd
}

// createAlgSpec is a constructor.  When called we assume that all
// the keys known to be associated with the specified algorithm in our
// tables are listed in the keyLen argument
func createAlgSpec(name string, keyLen []int) *AlgSpec {
	algSpec := new(AlgSpec)
	algSpec.Name = strings.ToLower(name)
	algSpec.KeyLen = keyLen
	return algSpec
}

// addCryptoAlg is called to include another algorithm
// in the CryptoDesc's list
func (cd *CryptoDesc) addAlgSpec(algSpec AlgSpec) {
	for _, as := range cd.Algs {
		if as.Name == algSpec.Name {
			return
		}
	}
	cd.Algs = append(cd.Algs, algSpec)
}

// WriteToFile stores the CryptoDesc struct to the file whose name is given.
// Serialization to json or to yaml is selected based on the extension of this name.
func (cd *CryptoDesc) WriteToFile(filename string) error {
	pathExt := path.Ext(filename)
	var bytes []byte
	var merr error = nil

	if pathExt == ".yaml" || pathExt == ".YAML" || pathExt == ".yml" {
		bytes, merr = yaml.Marshal(*cd)
	} else if pathExt == ".json" || pathExt == ".JSON" {
		bytes, merr = json.MarshalIndent(*cd, "", "\t")
	}

	if merr != nil {
		panic(merr)
	}

	f, cerr := os.Create(filename)
	if cerr != nil {
		panic(cerr)
	}
	_, werr := f.WriteString(string(bytes[:]))
	if werr != nil {
		panic(werr)
	}
	f.Close()
	return werr
}

// ReadCryptoDesc deserializes a byte slice holding a representation of an CryptoDesc struct.
// If the input argument of dict (those bytes) is empty, the file whose name is given is read
// to acquire them.  A deserialized representation is returned, or an error if one is generated
// from a file read or the deserialization.
func ReadCryptoDesc(filename string, useYAML bool, dict []byte) (*CryptoDesc, error) {
	var err error
	if len(dict) == 0 {
		dict, err = os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	example := CryptoDesc{}
	if useYAML {
		err = yaml.Unmarshal(dict, &example)
	} else {
		err = json.Unmarshal(dict, &example)
	}

	if err != nil {
		return nil, err
	}

	return &example, nil
}

// BuildCryptoDesc scans a given file of timing descriptions, looking for those
// with a Param == "KeyLength=integer" which are taken to be crypto algortithms and
// the Param a string-encoded integer description of the key length version
func BuildCryptoDesc(filename string) *CryptoDesc {

	cd := createCryptoDesc()

	var emptyBytes []byte
	fdl, _ := ReadFuncExecList(filename, true, emptyBytes)

	// create a temporary dictionary to more easily detect
	// when an algorithm has already been referenced
	algDict := make(map[string]AlgSpec)

	// look at each function timing description
	for identifier := range fdl.Times {
		for _, fd := range fdl.Times[identifier] {
			// check whether fd is a description of interest,
			// identified as having the param attribute have the format
			// keylength="integer"
			param := strings.ToLower(fd.Param)
			param = strings.Replace(param, " ", "", -1)
			if strings.Contains(param, "keylength=") {
				ps := strings.Split(param, "=")
				_, err := strconv.Atoi(ps[1])
				if err != nil {
					panic(fmt.Errorf("faulty parameter description for crypto timing"))
				}

				// crypto timings in the functional timings table are assumed to have the form
				// 'op-alg-keylen', and that neither op nor alg have the character '-' embedded,
				// so we can split on '-' to get the pieces
				lcName := strings.ToLower(fd.Identifier)
				cryptoParts := strings.Split(lcName, "-")

				if len(cryptoParts) != 3 {
					panic(fmt.Errorf("expect identifier in form of 'op-alg-keylen"))
				}

				// pull out notationally the algorithm and key lengths
				alg := cryptoParts[1]
				keyLen, _ := strconv.Atoi(cryptoParts[2])

				// see whether we've already seen this algorithm
				_, present := algDict[alg]

				if !present {
					// no, so create a representation for it
					algDict[alg] = AlgSpec{Name: alg, KeyLen: []int{}}
				}

				// add keylength, if not present
				as := algDict[alg]
				foundKey := false
				for _, kl := range as.KeyLen {
					if kl == keyLen {
						foundKey = true
						break
					}
				}
				if foundKey {
					continue
				}

				kls := algDict[alg].KeyLen
				kls = append(kls, keyLen)
				sort.Ints(kls)
				algDict[alg] = AlgSpec{Name: alg, KeyLen: kls}
			}
		}
	}
	// AlgDict is now a dictionary indexed by representation of
	// some crypto algorithm, with a list of key lengths that have been
	// observed as being declared in functional timings
	// Add these to the CryptoDesc
	for algName := range algDict {
		algName = strings.Replace(algName, "encrypt-", "", -1)
		algName = strings.Replace(algName, "decrypt-", "", -1)
		algName = strings.Replace(algName, "hash-", "", -1)
		algName = strings.Replace(algName, "sign-", "", -1)

		as := createAlgSpec(algName, algDict[algName].KeyLen)
		cd.addAlgSpec(*as)
	}
	// notice that there are no pointers embedded in cd,
	// so it can written out to file
	return cd
}
