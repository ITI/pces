module github.com/iti/pces

go 1.22.7

replace github.com/iti/mrnes => ../mrnes

require (
	github.com/iti/evt/evtm v0.1.4
	github.com/iti/evt/vrtime v0.1.5
	github.com/iti/mrnes v0.0.0-00010101000000-000000000000
	github.com/iti/rngstream v0.2.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/iti/evt/evtq v0.1.4 // indirect
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56 // indirect
	gonum.org/v1/gonum v0.15.0 // indirect
)
