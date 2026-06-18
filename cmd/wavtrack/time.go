package main

import (
	"fmt"
	"strings"
	"time"
)

type SampleTick int

func parseTimeInputWithBase(input string, base time.Duration) time.Duration {
	if len(input) > 0 && (input[0] == '+' || input[0] == '-') {
		// Relative time
		if duration, ok := parseTimeInput(input); ok {
			return max(base+duration, 0)
		}
	}
	// Absolute time
	if duration, ok := parseTimeInput(input); ok {
		return duration
	}
	return base
}

func sampleTickToDuration(st SampleTick, sampleRate int) time.Duration {
	if sampleRate <= 0 {
		return 0
	}
	seconds := float64(st) / float64(sampleRate)
	return time.Duration(seconds * float64(time.Second))
}

func parseTimeInputToSampleTick(input string, base SampleTick, sampleRate int) SampleTick {
	if hasColon := strings.Contains(input, ":"); !hasColon {
		// Parse as sample ticks
		var sign int = 1
		tok := input
		if len(tok) > 0 && (tok[0] == '+' || tok[0] == '-') {
			if tok[0] == '-' {
				sign = -1
			}
			tok = tok[1:]
		}
		var ticks int64
		_, err := fmt.Sscanf(tok, "%d", &ticks)
		if err == nil {
			return max(base+SampleTick(sign)*SampleTick(ticks), 0)
		}
	}

	// Parse as time (existing logic)
	baseDur := sampleTickToDuration(base, sampleRate)
	newDur := parseTimeInputWithBase(input, baseDur)
	if sampleRate <= 0 {
		return base
	}
	return SampleTick(float64(newDur) / float64(time.Second) * float64(sampleRate))
}

func FormatSampleTick(samples SampleTick, sampleRate int) string {
	if sampleRate <= 0 {
		return fmt.Sprintf("%d", samples)
	}
	seconds := float64(samples) / float64(sampleRate)
	dur := time.Duration(seconds * float64(time.Second))
	return formatDuration(dur)
}

func formatDuration(dur time.Duration) string {
	totalMs := max(int(dur.Milliseconds()), 0)
	hours := totalMs / 3600000
	minutes := (totalMs / 60000) % 60
	seconds := (totalMs / 1000) % 60
	milliseconds := totalMs % 1000
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
	}
	return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, milliseconds)
}

func parseTimeInput(input string) (time.Duration, bool) {
	var sign int = 1
	if len(input) > 0 && (input[0] == '+' || input[0] == '-') {
		if input[0] == '-' {
			sign = -1
		}
		input = input[1:]
	}

	var minutes, seconds, milliseconds int

	// Try MM:SS.mmmm
	var msPart string
	n, err := fmt.Sscanf(input, "%d:%d.%s", &minutes, &seconds, &msPart)
	if err == nil && n >= 2 {
		if seconds >= 60 {
			return 0, false
		}
		if len(msPart) > 4 {
			return 0, false
		}
		if len(msPart) > 0 {
			var msVal int
			_, err := fmt.Sscanf(msPart, "%d", &msVal)
			if err != nil {
				return 0, false
			}
			for i := len(msPart); i < 4; i++ {
				msVal *= 10
			}
			milliseconds = msVal
		}
		return time.Duration(sign) * (time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second + time.Duration(milliseconds)*time.Millisecond), true
	}

	// Try MM:SS
	n, err = fmt.Sscanf(input, "%d:%d", &minutes, &seconds)
	if err == nil && n == 2 {
		if seconds >= 60 {
			return 0, false
		}
		return time.Duration(sign) * (time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second), true
	}

	// Try SS.mmmm (treat as seconds + milliseconds)
	n, err = fmt.Sscanf(input, "%d.%s", &seconds, &msPart)
	if err == nil && n >= 1 {
		if len(msPart) > 4 {
			return 0, false
		}
		milliseconds = 0
		if len(msPart) > 0 {
			var msVal int
			_, err := fmt.Sscanf(msPart, "%d", &msVal)
			if err != nil {
				return 0, false
			}
			for i := len(msPart); i < 4; i++ {
				msVal *= 10
			}
			milliseconds = msVal
		}
		return time.Duration(sign) * (time.Duration(seconds)*time.Second + time.Duration(milliseconds)*time.Millisecond), true
	}

	return 0, false
}
