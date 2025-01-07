package midi

func Frame(data []byte) (msgs [][]byte, n int) {
	// Drop bytes missing message.
	var start int
	for start = 0; start < len(data); start++ {
		if IsMessage(data[start]) {
			break
		}
	}
	n = start
	for n < len(data) {
		if !IsMessage(data[start]) {
			panic("data start not message")
		}
		cmd := data[start]
		m := -1
		switch {
		case cmd == Start, cmd == Stop, cmd == Continue, cmd == Tick:
			m = 1
		case IsPgm(cmd):
			m = 2
		case IsCC(cmd), IsNoteOn(cmd), IsNoteOff(cmd), IsPitch(cmd):
			m = 3
			// TODO: sysex
			// TODO: running status
		}
		if m == -1 {
			// Unknown message; skip.
			n++
			break
		}
		if n+m > len(data) {
			break
		}
		msgs = append(msgs, data[start:start+m])
		n += m
		start = n
	}
	return msgs, n
}
