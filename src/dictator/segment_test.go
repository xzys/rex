package dictator

import (
	"encoding/binary"
	"testing"
	"time"
)

// pcmChunk builds dur of S16_LE mono PCM with a square wave of the given amplitude.
func pcmChunk(dur time.Duration, amplitude int16) []byte {
	n := int(dur.Seconds() * sampleRate)
	buf := make([]byte, n*2)
	for i := 0; i < n; i++ {
		v := amplitude
		if i%2 == 0 {
			v = -amplitude
		}
		binary.LittleEndian.PutUint16(buf[2*i:], uint16(v))
	}
	return buf
}

func feedFor(t *testing.T, s *segmenter, dur time.Duration, amplitude int16) []byte {
	t.Helper()
	var seg []byte
	for elapsed := time.Duration(0); elapsed < dur; elapsed += drainInterval {
		if got := s.Feed(pcmChunk(drainInterval, amplitude)); got != nil {
			if seg != nil {
				t.Fatal("got two segments from one utterance")
			}
			seg = got
		}
	}
	return seg
}

func TestSegmenterCutsOnSilence(t *testing.T) {
	s := newSegmenter()

	if seg := feedFor(t, s, 2*time.Second, 20); seg != nil {
		t.Fatal("silence alone should not produce a segment")
	}
	if seg := feedFor(t, s, 1*time.Second, 8000); seg != nil {
		t.Fatal("should not cut while speech is ongoing")
	}
	seg := feedFor(t, s, 1200*time.Millisecond, 20)
	if seg == nil {
		t.Fatal("expected a segment after trailing silence")
	}
	// Segment should hold roughly the 1s of speech plus pre-roll and trailing
	// silence — at least the speech itself.
	if got := len(seg); got < bytesPerSecond {
		t.Fatalf("segment too short: %d bytes", got)
	}

	// After a cut the segmenter starts fresh: more silence yields nothing.
	if seg := feedFor(t, s, 1*time.Second, 20); seg != nil {
		t.Fatal("no new segment expected after cut during silence")
	}
}

func TestSegmenterForceCutsLongSpeech(t *testing.T) {
	s := newSegmenter()
	seg := feedFor(t, s, maxSegment+time.Second, 8000)
	if seg == nil {
		t.Fatal("expected force-cut of continuous speech at maxSegment")
	}
}

func TestSegmenterFlush(t *testing.T) {
	s := newSegmenter()
	feedFor(t, s, 500*time.Millisecond, 8000)
	if seg := s.Flush(); seg == nil {
		t.Fatal("flush should return in-progress speech")
	}
	if seg := s.Flush(); seg != nil {
		t.Fatal("second flush should be empty")
	}

	feedFor(t, s, 500*time.Millisecond, 20)
	if seg := s.Flush(); seg != nil {
		t.Fatal("flush of pure silence should be empty")
	}
}

func TestSegmenterPreRollBounded(t *testing.T) {
	s := newSegmenter()
	feedFor(t, s, 5*time.Second, 20)
	if keep := int(preRoll.Seconds()*bytesPerSecond) + len(pcmChunk(drainInterval, 0)); len(s.buf) > keep {
		t.Fatalf("silent buffer not bounded: %d bytes", len(s.buf))
	}
}
