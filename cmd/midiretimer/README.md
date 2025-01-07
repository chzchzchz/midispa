# midiretimer

Stalls MIDI clock events until appropriate time has elapsed.

## websockets

Use websocketd for websockets with webmidi:
```sh
websocketd -binary -port 8081 -- ./midiretimer --out-port 20:0 2>&1
```