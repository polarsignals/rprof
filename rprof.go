package rprof

import (
	"errors"
	"io"
	"runtime"
	"sync"
	"time"

	proto "go.opentelemetry.io/proto/otlp/profiles/v1experimental"
)

var (
	// profiler is the default profiler used by the package-level functions.
	profiler = NewProfiler()
)

// Start starts the default profiler.
func Start() error {
	return profiler.Start()
}

// Stop stops the default profiler and returns the profile.
func Stop() (*proto.Profile, error) {
	return profiler.Stop()
}

// Reader returns a new io.Reader that will be profiled if the profiler is on.
func Reader(r io.Reader) io.Reader {
	return profiler.Reader(r)
}

// ReadCloser returns a new io.ReadCloser that will be profiled if the profiler is on.
func ReadCloser(r io.ReadCloser) io.ReadCloser {
	return profiler.ReadCloser(r)
}

// ReaderAt returns a new io.ReaderAt that will be profiled if the profiler is on.
func ReaderAt(r io.ReaderAt) io.ReaderAt {
	return profiler.ReaderAt(r)
}

// sampleKey is the key used to group a unique sample. If the same stack and
// size bucket are seen multiple times then the values are aggregated.
type sampleKey struct {
	locations       [128]uintptr
	sizeBucketPower uint8
	numLocations    uint8
}

// Rprof is a profiler that records the number of reads and the number of bytes
// read since the last call to Start.
type Rprof struct {
	mu        sync.Mutex
	samples   map[sampleKey][2]int64
	startTime int64
}

// Start starts the profiler. If the profiler is already started then it returns an error.
func (p *Rprof) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.startTime != 0 {
		return errors.New("profiler already started")
	}

	p.startTime = time.Now().UnixNano()
	p.samples = map[sampleKey][2]int64{}

	return nil
}

// profileBuilder is a helper to build a profile.
type profileBuilder struct {
	p *proto.Profile
}

// newProfileBuilder returns a new profileBuilder with the given timestamp and duration.
func newProfileBuilder(timestampNanos, durationNanos int64) *profileBuilder {
	b := &profileBuilder{
		p: &proto.Profile{
			// StringTable is initialized with values we know are going to be there.
			StringTable: []string{
				"",
				"reads",
				"count",
				"read",
				"bytes",
			},
			DurationNanos: durationNanos,
			TimeNanos:     timestampNanos,
			Period:        1,
			PeriodType: &proto.ValueType{
				Type: 1, // "reads" in the string table
				Unit: 2, // "count" in the string table
			},
			SampleType: []*proto.ValueType{{
				Type: 1, // "reads" in the string table
				Unit: 2, // "count" in the string table
			}, {
				Type: 3, // "read" in the string table
				Unit: 4, // "bytes" in the string table
			}},
		},
	}

	// populate the mappings right away
	b.readMapping()
	return b
}

// addString adds a string to the string table and returns the index.
func (b *profileBuilder) addString(s string) int64 {
	b.p.StringTable = append(b.p.StringTable, s)
	return int64(len(b.p.StringTable)) - 1
}

// addMapping is called from the respective platform-specific implementations
// to add a mapping to the profile.
func (b *profileBuilder) addMapping(lo, hi, offset uint64, file, buildID string) {
	b.addMappingEntry(lo, hi, offset, file, buildID, false)
}

// addMappingEntry adds a mapping to the profile. If it's a fake mapping then a
// mapping that covers the entire address space is added.
func (b *profileBuilder) addMappingEntry(lo, hi, offset uint64, file, buildID string, fake bool) {
	nextIdx := uint64(len(b.p.Mapping)) + 1
	if fake {
		b.p.Mapping = append(b.p.Mapping, &proto.Mapping{
			Id:          nextIdx,
			MemoryStart: 0,
			MemoryLimit: 1 << 63,
			FileOffset:  0,
			Filename:    0,
			BuildId:     0,
		})
		return
	}

	b.p.Mapping = append(b.p.Mapping, &proto.Mapping{
		Id:          nextIdx,
		MemoryStart: lo,
		MemoryLimit: hi,
		FileOffset:  offset,
		Filename:    b.addString(file),
		BuildId:     b.addString(buildID),
	})
}

// build populates the samples and locations in the profile.
func (b *profileBuilder) build(samples map[sampleKey][2]int64) *proto.Profile {
	b.p.Sample = make([]*proto.Sample, 0, len(samples))

	locIdx := map[uintptr]uint64{}
	locs := make([]uint64, 0, 128)

	for sampleKey, sampleValue := range samples {
		locs = locs[:0]

		for _, loc := range sampleKey.locations {
			idx, ok := locIdx[loc]
			if !ok {
				idx = uint64(len(locIdx)) + 1
				locIdx[loc] = idx

				var mappingId uint64
				addr := uint64(loc)
				for i := uint64(0); i < uint64(len(b.p.Mapping)); i++ {
					if b.p.Mapping[i].MemoryStart <= addr && addr < b.p.Mapping[i].MemoryLimit {
						mappingId = i + 1 // IDs are 1-indexed
						break
					}
				}

				b.p.Location = append(b.p.Location, &proto.Location{
					Id:           idx,
					MappingIndex: mappingId,
					Address:      uint64(addr),
				})
			}

			locs = append(locs, idx)
		}

		b.p.Sample = append(b.p.Sample, &proto.Sample{
			// Copy the locations since we're reusing the slice.
			LocationIndex: copyLocs(locs),
			Value:         sampleValue[:],
			Label: []*proto.Label{{
				Key: 4, // "bytes"
				Num: 1 << sampleKey.sizeBucketPower,
			}},
		})
	}

	return b.p
}

// copyLocs copies the locations to a new slice.
func copyLocs(locs []uint64) []uint64 {
	res := make([]uint64, len(locs))
	copy(res, locs)
	return res
}

// Stop stops the profiler and returns the profile. If the profiler is not
// started then it returns an error.
func (p *Rprof) Stop() (*proto.Profile, error) {
	p.mu.Lock()

	if p.startTime == 0 {
		p.mu.Unlock()
		return nil, errors.New("profiler not started")
	}

	ts := p.startTime
	samples := p.samples

	p.startTime = 0
	p.mu.Unlock()

	duration := time.Now().UnixNano() - ts

	b := newProfileBuilder(ts, duration)
	return b.build(samples), nil
}

func (p *Rprof) recordSample(size int) {
	sizeBucketPower := nextPowerOfTwo(size)

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.startTime == 0 {
		// profiler not started
		return
	}

	locations := [128]uintptr{}
	numRead := runtime.Callers(3, locations[:])

	k := sampleKey{
		locations:       locations,
		numLocations:    uint8(numRead),
		sizeBucketPower: sizeBucketPower,
	}
	sample := p.samples[k]

	// first sample is the number of reads
	sample[0]++

	// second sample is the number of bytes read
	sample[1] += int64(size)

	p.samples[k] = sample
}

// nextPowerOfTwo returns the next power of two that is greater or equal to the input. It returns the power, not the value to be able to return a uint8.
func nextPowerOfTwo(input int) uint8 {
	for i := 0; i < 63; i++ {
		if 1<<i >= input {
			return uint8(i)
		}
	}
	return 63
}

// NewProfiler returns a new profiler.
func NewProfiler() *Rprof {
	return &Rprof{}
}

// RprofReader is an io.Reader that will profile the reads if the profiler is on.
type RprofReader struct {
	p *Rprof
	r io.Reader
}

// Reader returns a new io.Reader that will be profiled if the profiler is on.
func (p *Rprof) Reader(r io.Reader) io.Reader {
	return &RprofReader{
		p: p,
		r: r,
	}
}

// Read reads from the underlying reader and records the sample in the profiler.
// Implements io.Reader.
func (r *RprofReader) Read(buf []byte) (int, error) {
	n, err := r.r.Read(buf)
	r.p.recordSample(n)
	return n, err
}

// RprofReadCloser is an io.ReadCloser that will profile the reads if the profiler is on.
type RprofReadCloser struct {
	p *Rprof
	r io.ReadCloser
}

// ReadCloser returns a new io.ReadCloser that will be profiled if the profiler is on.
func (p *Rprof) ReadCloser(r io.ReadCloser) io.ReadCloser {
	return &RprofReadCloser{
		p: p,
		r: r,
	}
}

// Read reads from the underlying reader and records the sample in the profiler.
// Implements io.Reader.
func (r *RprofReadCloser) Read(buf []byte) (int, error) {
	n, err := r.r.Read(buf)
	r.p.recordSample(n)
	return n, err
}

// Close closes the underlying reader.
// Implements io.Closer.
func (r *RprofReadCloser) Close() error {
	return r.r.Close()
}

// RprofReaderAt is an io.ReaderAt that will profile the reads if the profiler is on.
type RprofReaderAt struct {
	p *Rprof
	r io.ReaderAt
}

// ReaderAt returns a new io.ReaderAt that will be profiled if the profiler is on.
func (p *Rprof) ReaderAt(r io.ReaderAt) io.ReaderAt {
	return &RprofReaderAt{
		p: p,
		r: r,
	}
}

// ReadAt reads from the underlying reader and records the sample in the profiler.
func (r *RprofReaderAt) ReadAt(buf []byte, off int64) (int, error) {
	n, err := r.r.ReadAt(buf, off)
	r.p.recordSample(n)
	return n, err
}
