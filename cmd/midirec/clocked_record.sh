#!/bin/bash

mkdir -p output

killall midiclock midirec midifilter midiloop

../midifilter/midifilter  -bpf ../midifilter/examples/song_select.elf  -i "W-BW61 MIDI 1" &
pids="$!"
../midiclock/midiclock -bpm 120 &
pids="$! $pids"

# wait for inputs to launch
sleep 1s


../midiloop/midiloop -i midifilter -c midiclock -dir output/slots -o "MIDI4x4 MIDI Out 1" &
./midirec -q -i midifilter -m midiclock -o output/ &
pids="$! $pids"

clock_port=`aconnect -l | grep midiclock | cut -f1 -d: | awk ' {print $2 } ' | head -n1`
midi_port=`aconnect -l | grep MIDI4x4 | cut -f1 -d: | awk ' { print $2 } ' | head -n1`

aconnect $clock_port:0 $midi_port:2

wait $pids