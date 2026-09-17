package alsa

import "testing"

const outputAllocationRuns = 100

func TestOutputAllocations(t *testing.T) {
	seq := newTestSeq()
	for _, message := range supportedMessages {
		t.Run(message.name, func(t *testing.T) {
			event := MakeEvent(message.data)
			allocations := testing.AllocsPerRun(outputAllocationRuns, func() {
				if err := seq.Write(event); err != nil {
					t.Fatal(err)
				}
			})
			if allocations > 1 {
				t.Fatalf("allocations per write = %v, want at most 1", allocations)
			}
		})
	}
}

func BenchmarkOutput(b *testing.B) {
	seq, err := OpenSeq("output-benchmark")
	if err != nil {
		b.Skipf("ALSA sequencer unavailable: %v", err)
	}
	defer seq.Close()
	for _, message := range supportedMessages {
		b.Run(message.name, func(b *testing.B) {
			event := MakeEvent(message.data)
			b.ReportAllocs()
			for index := 0; index < b.N; index++ {
				if err := seq.Write(event); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
