#!/bin/bash

set -eou pipefail

function f {
	expr $RANDOM % 233
}
function pan {
	expr $RANDOM % 7
}
function t {
	expr $RANDOM % 8
}

kit='[{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'},
{"SoundNumber" : '`f`', "Volume" : 90, "Panning" : '`pan`', "Tuning": '`t`'}]'

echo $kit

dev="MIDI4x4%20MIDI%201"
curl "http://localhost:4567/$dev/sysex/sr16/DrumSet" -XPOST -d"$kit"
