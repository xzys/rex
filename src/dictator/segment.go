package dictator

import (
	"encoding/binary"
	"math"
	"time"
)

const (
	sampleRate     = 16000
	bytesPerSecond = sampleRate * 2 // S16_LE mono

	// silenceCut is how much trailing silence ends an utterance.
	silenceCut = 700 * time.Millisecond
	// maxSegment force-cuts a segment so long monologues still stream out.
	maxSegment = 15 * time.Second
	// preRoll is how much leading audio to keep while waiting for speech,
	// so the first syllable isn't clipped.
	preRoll = 250 * time.Millisecond
)

// segmenter accumulates raw PCM and cuts utterance-sized segments: a segment
// ends after silenceCut of trailing quiet, or at maxSegment. While no speech
// has been detected it keeps only a preRoll's worth of audio.
type segmenter struct {
	buf        []byte
	hasSpeech  bool
	silence    time.Duration
	noiseFloor float64
	lastRMS    float64
}

func newSegmenter() *segmenter {
	return &segmenter{noiseFloor: 200}
}

// threshold is the RMS level above which a chunk counts as speech. It tracks
// the ambient noise floor so the cut point adapts to mic gain and room noise.
func (s *segmenter) threshold() float64 {
	t := s.noiseFloor * 2.5
	return math.Min(math.Max(t, 300), 3000)
}

// Feed appends a chunk of captured PCM and returns a completed segment, or
// nil if the current utterance is still in progress.
func (s *segmenter) Feed(chunk []byte) []byte {
	if len(chunk) == 0 {
		return nil
	}
	r := rms(chunk)
	s.lastRMS = r
	dur := time.Duration(len(chunk)) * time.Second / bytesPerSecond
	loud := r > s.threshold()
	if !loud {
		s.noiseFloor = 0.95*s.noiseFloor + 0.05*r
	}

	s.buf = append(s.buf, chunk...)
	if loud {
		s.hasSpeech = true
		s.silence = 0
	} else {
		s.silence += dur
	}

	if s.hasSpeech && (s.silence >= silenceCut || len(s.buf) >= int(maxSegment.Seconds())*bytesPerSecond) {
		return s.cut()
	}
	if !s.hasSpeech {
		if keep := int(preRoll.Seconds() * bytesPerSecond); len(s.buf) > keep {
			s.buf = append([]byte(nil), s.buf[len(s.buf)-keep:]...)
		}
	}
	return nil
}

// Flush returns the in-progress utterance if it contains speech (for pause /
// exit), discarding anything else.
func (s *segmenter) Flush() []byte {
	if !s.hasSpeech {
		s.buf = nil
		return nil
	}
	return s.cut()
}

func (s *segmenter) cut() []byte {
	seg := s.buf
	s.buf = nil
	s.hasSpeech = false
	s.silence = 0
	return seg
}

// Level returns the most recent chunk's RMS normalized to 0..1 for the meter.
func (s *segmenter) Level() float64 {
	return math.Min(s.lastRMS/8000, 1)
}

func rms(pcm []byte) float64 {
	n := len(pcm) / 2
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i < n; i++ {
		v := float64(int16(binary.LittleEndian.Uint16(pcm[2*i:])))
		sum += v * v
	}
	return math.Sqrt(sum / float64(n))
}
