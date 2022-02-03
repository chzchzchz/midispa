#!/bin/bash
set -eou pipefail

dev="MIDI4x4%20MIDI%202"
#curl "http://localhost:4567/$dev/sysex/sr16/DrumSetRequest" -XPOST -d"{}"

#curl --header "Accept: application/json; content=\"sysex/SysEx\"" 
#	"http://localhost:4567/$dev/sysex/sr16/DumpRequest" -XPOST -d"{}"

curl --header "Accept: application/json; content=\"sysex/sr16/DrumSet\"" \
	"http://localhost:4567/$dev/sysex/sr16/DrumSetRequest" -XPOST -d"{}"
