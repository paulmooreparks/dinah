package responsiveness

import (
	"math"
	"runtime/metrics"
	"time"
)

const (
	valueUnavailable uint8 = iota
	valueMeasured
	valueUnsupported
)
const histogramCapacity = 256

func runtimeNumber(state uint8, value float64) Number {
	if state == valueUnsupported {
		return missing(NotSupported)
	}
	if state != valueMeasured || math.IsInf(value, 0) || math.IsNaN(value) || value < 0 {
		return missing(Unavailable)
	}
	return number(value)
}

type runtimeReader struct {
	samples    [2]metrics.Sample
	counts     [histogramCapacity]uint64
	buckets    [histogramCapacity + 1]float64
	length     int
	mutex      float64
	mutexValid bool
	previous   time.Time
}

func (r *runtimeReader) read(now, anchor time.Time) runtimeValue {
	r.samples[0].Name = "/sched/pauses/total/gc:seconds"
	r.samples[1].Name = "/sync/mutex/wait/total:seconds"
	metrics.Read(r.samples[:])
	v := runtimeValue{}
	if !r.previous.IsZero() {
		v.start = r.previous.Sub(anchor)
		v.duration = now.Sub(r.previous)
	}
	gc := r.samples[0].Value
	if gc.Kind() != metrics.KindFloat64Histogram {
		v.countState = valueUnsupported
		v.maximumState = valueUnsupported
	} else {
		h := gc.Float64Histogram()
		if len(h.Counts) > histogramCapacity || len(h.Buckets) != len(h.Counts)+1 {
			v.countState = valueUnsupported
			v.maximumState = valueUnsupported
		} else {
			compatible := r.length == len(h.Counts) && !r.previous.IsZero()
			if compatible {
				for i, b := range h.Buckets {
					if b != r.buckets[i] {
						compatible = false
						break
					}
				}
			}
			if !r.previous.IsZero() && !compatible {
				v.countState = valueUnsupported
				v.maximumState = valueUnsupported
			}
			if compatible {
				v.countState = valueMeasured
				for i, count := range h.Counts {
					if count < r.counts[i] {
						v.countState = valueUnavailable
						v.maximumState = valueUnavailable
						break
					}
					delta := count - r.counts[i]
					v.count += float64(delta)
					if delta != 0 {
						v.maximumState = valueMeasured
						v.lower = h.Buckets[i] * 1000
						v.upper = h.Buckets[i+1] * 1000
						v.open = math.IsInf(v.upper, 1)
					}
				}
			}
			copy(r.counts[:], h.Counts)
			copy(r.buckets[:], h.Buckets)
			r.length = len(h.Counts)
		}
	}
	m := r.samples[1].Value
	if m.Kind() != metrics.KindFloat64 {
		v.mutexState = valueUnsupported
		r.mutexValid = false
	} else {
		value := m.Float64()
		if r.mutexValid && value >= r.mutex {
			v.mutexState = valueMeasured
			v.mutex = (value - r.mutex) * 1000
		}
		r.mutex = value
		r.mutexValid = true
	}
	r.previous = now
	return v
}
