package player

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"unsafe"
)

// =============================================================================
// Benchmark 1: byte→int16 conversion (old loop vs new unsafe.Slice)
// =============================================================================

// Old implementation: per-element loop conversion
func convertBytesToInt16Loop(buf []byte, dst []int16) {
	n := len(dst)
	for j := range n {
		dst[j] = int16(buf[j*2]) | (int16(buf[j*2+1]) << 8)
	}
}

// New implementation: zero-copy unsafe.Slice reinterpretation
func convertBytesToInt16Unsafe(buf []byte) []int16 {
	n := len(buf) / 2
	return unsafe.Slice((*int16)(unsafe.Pointer(&buf[0])), n)
}

func BenchmarkConvertBytesToInt16(b *testing.B) {
	for _, size := range []int{1024, 4096, 16384, 65536, 262144} {
		buf := make([]byte, size)
		for i := range buf {
			buf[i] = byte(rand.IntN(256))
		}
		dst := make([]int16, size/2)

		b.Run("Loop/"+formatSize(size), func(b *testing.B) {
			for b.Loop() {
				convertBytesToInt16Loop(buf, dst)
			}
		})

		b.Run("UnsafeSlice/"+formatSize(size), func(b *testing.B) {
			var result []int16
			for b.Loop() {
				result = convertBytesToInt16Unsafe(buf)
			}
			_ = result
		})
	}
}

// =============================================================================
// Benchmark 2: int16→float64 sample conversion (per-sample vs batch)
// =============================================================================

// Old implementation: per-sample mono conversion
func fillMonoSampleOld(buffer []int16, bufferIndex int, samples [][2]float64) int {
	n := 0
	for i := range samples {
		if bufferIndex >= len(buffer) {
			break
		}
		sample := float64(buffer[bufferIndex]) / 32768.0
		samples[i][0] = sample
		samples[i][1] = sample
		bufferIndex++
		n++
	}
	return n
}

// New implementation: batch mono conversion
func fillMonoBatchNew(buffer []int16, bufferIndex int, samples [][2]float64) int {
	available := len(buffer) - bufferIndex
	count := min(available, len(samples))

	const scale = 1.0 / 32768.0
	buf := buffer[bufferIndex : bufferIndex+count]
	for i, s := range buf {
		v := float64(s) * scale
		samples[i][0] = v
		samples[i][1] = v
	}
	return count
}

// Old implementation: per-sample stereo conversion
func fillStereoSampleOld(buffer []int16, bufferIndex int, samples [][2]float64) int {
	n := 0
	for i := range samples {
		if bufferIndex+1 >= len(buffer) {
			break
		}
		left := float64(buffer[bufferIndex]) / 32768.0
		right := float64(buffer[bufferIndex+1]) / 32768.0
		samples[i][0] = left
		samples[i][1] = right
		bufferIndex += 2
		n++
	}
	return n
}

// New implementation: batch stereo conversion
func fillStereoBatchNew(buffer []int16, bufferIndex int, samples [][2]float64) int {
	availablePairs := (len(buffer) - bufferIndex) / 2
	count := min(availablePairs, len(samples))

	const scale = 1.0 / 32768.0
	idx := bufferIndex
	for i := range count {
		samples[i][0] = float64(buffer[idx]) * scale
		samples[i][1] = float64(buffer[idx+1]) * scale
		idx += 2
	}
	return count
}

func BenchmarkFillMono(b *testing.B) {
	for _, size := range []int{512, 2048, 8192, 32768} {
		buffer := make([]int16, size)
		for i := range buffer {
			buffer[i] = int16(rand.IntN(65536) - 32768)
		}
		samples := make([][2]float64, size)

		b.Run("PerSample/"+formatSize(size), func(b *testing.B) {
			for b.Loop() {
				fillMonoSampleOld(buffer, 0, samples)
			}
		})

		b.Run("Batch/"+formatSize(size), func(b *testing.B) {
			for b.Loop() {
				fillMonoBatchNew(buffer, 0, samples)
			}
		})
	}
}

func BenchmarkFillStereo(b *testing.B) {
	for _, size := range []int{512, 2048, 8192, 32768} {
		buffer := make([]int16, size*2) // interleaved L/R
		for i := range buffer {
			buffer[i] = int16(rand.IntN(65536) - 32768)
		}
		samples := make([][2]float64, size)

		b.Run("PerSample/"+formatSize(size), func(b *testing.B) {
			for b.Loop() {
				fillStereoSampleOld(buffer, 0, samples)
			}
		})

		b.Run("Batch/"+formatSize(size), func(b *testing.B) {
			for b.Loop() {
				fillStereoBatchNew(buffer, 0, samples)
			}
		})
	}
}

// =============================================================================
// Benchmark 3: Ring buffer write/read (per-element vs copy)
// =============================================================================

// Old implementation: per-element ring buffer write
func ringBufferWriteOld(buffer [][2]float64, writePos, bufferSize int, src [][2]float64) int {
	for i := range src {
		buffer[writePos] = src[i]
		writePos = (writePos + 1) % bufferSize
	}
	return writePos
}

// New implementation: copy-based ring buffer write
func ringBufferWriteNew(buffer [][2]float64, writePos, bufferSize int, src [][2]float64) int {
	n := len(src)
	firstChunk := bufferSize - writePos
	if firstChunk >= n {
		copy(buffer[writePos:], src[:n])
		writePos = (writePos + n) % bufferSize
	} else {
		copy(buffer[writePos:], src[:firstChunk])
		copy(buffer, src[firstChunk:n])
		writePos = n - firstChunk
	}
	return writePos
}

// Old implementation: per-element ring buffer read
func ringBufferReadOld(buffer [][2]float64, readPos, bufferSize, filled int, dst [][2]float64) (int, int, int) {
	n := 0
	for i := range dst {
		if filled == 0 {
			return n, readPos, filled
		}
		dst[i] = buffer[readPos]
		readPos = (readPos + 1) % bufferSize
		filled--
		n++
	}
	return n, readPos, filled
}

// New implementation: copy-based ring buffer read
func ringBufferReadNew(buffer [][2]float64, readPos, bufferSize, filled int, dst [][2]float64) (int, int, int) {
	toRead := min(filled, len(dst))
	firstChunk := bufferSize - readPos
	if firstChunk >= toRead {
		copy(dst, buffer[readPos:readPos+toRead])
		readPos = (readPos + toRead) % bufferSize
	} else {
		copy(dst, buffer[readPos:readPos+firstChunk])
		copy(dst[firstChunk:], buffer[:toRead-firstChunk])
		readPos = toRead - firstChunk
	}
	filled -= toRead
	return toRead, readPos, filled
}

func BenchmarkRingBufferWrite(b *testing.B) {
	for _, size := range []int{512, 2048, 8192, 32768} {
		bufferSize := size * 4
		buffer := make([][2]float64, bufferSize)
		src := make([][2]float64, size)
		for i := range src {
			src[i] = [2]float64{rand.Float64(), rand.Float64()}
		}

		b.Run("PerElement/"+formatSize(size), func(b *testing.B) {
			writePos := 0
			for b.Loop() {
				writePos = ringBufferWriteOld(buffer, writePos, bufferSize, src)
			}
		})

		b.Run("Copy/"+formatSize(size), func(b *testing.B) {
			writePos := 0
			for b.Loop() {
				writePos = ringBufferWriteNew(buffer, writePos, bufferSize, src)
			}
		})
	}
}

func BenchmarkRingBufferRead(b *testing.B) {
	for _, size := range []int{512, 2048, 8192, 32768} {
		bufferSize := size * 4
		buffer := make([][2]float64, bufferSize)
		for i := range buffer {
			buffer[i] = [2]float64{rand.Float64(), rand.Float64()}
		}
		dst := make([][2]float64, size)

		b.Run("PerElement/"+formatSize(size), func(b *testing.B) {
			for b.Loop() {
				ringBufferReadOld(buffer, 0, bufferSize, size, dst)
			}
		})

		b.Run("Copy/"+formatSize(size), func(b *testing.B) {
			for b.Loop() {
				ringBufferReadNew(buffer, 0, bufferSize, size, dst)
			}
		})
	}
}

// formatSize returns a human-readable size string.
func formatSize(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%dM", n/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%dK", n/1024)
	default:
		return fmt.Sprintf("%d", n)
	}
}
