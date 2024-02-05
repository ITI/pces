module main

replace github.com/iti/mrnesbits => ../../../mrnesbits

replace github.com/iti/mrnes => ../../../mrnes

go 1.19

require (
	github.com/iti/cmdline v0.1.0
	github.com/iti/mrnes v0.0.0-20240201162834-c8478a0afcc9
	github.com/iti/mrnesbits v0.0.0-00010101000000-000000000000
)

require (
	github.com/iti/evt v0.1.0 // indirect
	github.com/iti/rngstream v0.2.2 // indirect
	golang.org/x/exp v0.0.0-20240119083558-1b970713d09a // indirect
	gonum.org/v1/gonum v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
