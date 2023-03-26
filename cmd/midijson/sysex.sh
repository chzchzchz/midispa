#!/bin/bash
set -eou pipefail

dev="MIDI4x4%20MIDI%20Out%204"
#curl "http://localhost:4567/$dev/sysex/sr16/DrumSetRequest" -XPOST -d"{}"

#curl --header "Accept: application/json; content=\"sysex/SysEx\""  \
#	"http://localhost:4567/$dev/sysex/SysEx" -XGET


curl --header "Accept: application/octet-stream; content=\"sysex/SysEx\""  \
	"http://localhost:4567/$dev/sysex/SysEx" -XGET \
	--output out.sysex

#curl --header "Accept: application/json; content=\"sysex/SysEx\""  \
#	"http://localhost:4567/$dev/sysex/sr16/DumpRequest" -XPOST -d"{}"

#curl --header "Accept: application/json; content=\"sysex/sr16/DrumSet\"" \
#	"http://localhost:4567/$dev/sysex/sr16/DrumSetRequest" -XPOST -d"{}"
