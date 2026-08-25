//go:build darwin

package stats

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestSamplerSampleResourcesUsesDarwinHostCounters(t *testing.T) {
	sampler := &Sampler{}
	first, err := sampler.SampleResources(nil)
	if err != nil {
		t.Fatalf("first Darwin resource sample: %v", err)
	}
	if first.Header.MemoryBytes == 0 {
		t.Fatalf("first Darwin resource sample has no host memory: %#v", first.Header)
	}
	time.Sleep(25 * time.Millisecond)
	second, err := sampler.SampleResources(nil)
	if err != nil {
		t.Fatalf("second Darwin resource sample: %v", err)
	}
	if !second.Ready {
		t.Fatalf("second Darwin resource sample not ready: %#v", second)
	}
	if !second.Header.CPUValid {
		t.Fatalf("second Darwin resource sample has no CPU delta: %#v", second.Header)
	}
}

func TestParseDarwinCPUTime(t *testing.T) {
	for _, testCase := range []struct {
		value string
		want  uint64
	}{
		{value: "00:01.25", want: 125},
		{value: "103:20.39", want: 620039},
		{value: "1:02:03.04", want: 372304},
		{value: "2-01:02:03.04", want: 17652304},
	} {
		got, err := parseDarwinCPUTime(testCase.value)
		if err != nil || got != testCase.want {
			t.Errorf("parseDarwinCPUTime(%q) = %d, %v; want %d", testCase.value, got, err, testCase.want)
		}
	}
}

func TestParseDarwinSwapXSWUsage(t *testing.T) {
	record := make([]byte, 32)
	binary.LittleEndian.PutUint64(record[:8], 7<<30)
	binary.LittleEndian.PutUint64(record[8:16], 2<<30)
	binary.LittleEndian.PutUint64(record[16:24], 5<<30)
	total, used, err := parseDarwinSwap(record)
	if err != nil || total != 7<<30 || used != 5<<30 {
		t.Fatalf("parseDarwinSwap() = total:%d used:%d err:%v", total, used, err)
	}
}
