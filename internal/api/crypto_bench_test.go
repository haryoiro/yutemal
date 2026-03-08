package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/pbkdf2"
)

// =============================================================================
// Benchmark: SHA1 (used in SAPISIDHASH computation per API request)
// =============================================================================

func BenchmarkSHA1(b *testing.B) {
	data := []byte("1234567890 SAPISID_VALUE https://music.youtube.com")

	b.Run("SHA1", func(b *testing.B) {
		for b.Loop() {
			h := sha1.New() //nolint:gosec
			h.Write(data)
			h.Sum(nil)
		}
	})
}

// =============================================================================
// Benchmark: SHA256 (used in cache key generation)
// =============================================================================

func BenchmarkSHA256(b *testing.B) {
	for _, size := range []int{64, 256, 1024, 4096} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i)
		}

		b.Run(fmt.Sprintf("%dB", size), func(b *testing.B) {
			for b.Loop() {
				sha256.Sum256(data)
			}
		})
	}
}

// =============================================================================
// Benchmark: PBKDF2 (used in browser cookie decryption key derivation)
// =============================================================================

func BenchmarkPBKDF2(b *testing.B) {
	password := []byte("chrome-safe-storage-key")
	salt := []byte("saltysalt")

	b.Run("1003iterations", func(b *testing.B) {
		for b.Loop() {
			pbkdf2.Key(password, salt, 1003, 16, sha1.New)
		}
	})
}

// =============================================================================
// Benchmark: AES-128-CBC decrypt (used in cookie decryption)
// =============================================================================

func BenchmarkAESCBCDecrypt(b *testing.B) {
	key := make([]byte, 16) // AES-128
	for i := range key {
		key[i] = byte(i)
	}
	iv := make([]byte, aes.BlockSize)
	for i := range iv {
		iv[i] = ' '
	}

	for _, blocks := range []int{1, 4, 16, 64} {
		size := blocks * aes.BlockSize
		encrypted := make([]byte, size)
		for i := range encrypted {
			encrypted[i] = byte(i)
		}
		decrypted := make([]byte, size)

		block, _ := aes.NewCipher(key)

		b.Run(fmt.Sprintf("%dblocks", blocks), func(b *testing.B) {
			for b.Loop() {
				mode := cipher.NewCBCDecrypter(block, iv)
				mode.CryptBlocks(decrypted, encrypted)
			}
		})
	}
}

// =============================================================================
// Benchmark: SAPISIDHASH full computation (real-world per-request cost)
// =============================================================================

func BenchmarkSAPISIDHash(b *testing.B) {
	sapisid := "SAPISID_1234567890abcdef"
	domain := "https://music.youtube.com"

	b.Run("FullComputation", func(b *testing.B) {
		for b.Loop() {
			data := fmt.Sprintf("%d %s %s", 1234567890, sapisid, domain)
			h := sha1.New() //nolint:gosec
			h.Write([]byte(data))
			_ = fmt.Sprintf("%d_%x", 1234567890, h.Sum(nil))
		}
	})
}

// =============================================================================
// Benchmark: sync.Mutex (LSE atomics improve mutex performance)
// =============================================================================

func BenchmarkMutex(b *testing.B) {
	b.Run("Uncontended", func(b *testing.B) {
		var mu sync.Mutex
		for b.Loop() {
			mu.Lock()
			mu.Unlock()
		}
	})

	b.Run("Contended/4goroutines", func(b *testing.B) {
		var mu sync.Mutex
		var counter int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		})
	})
}

// =============================================================================
// Benchmark: sync.RWMutex (used in download.go inProgress tracking)
// =============================================================================

func BenchmarkRWMutex(b *testing.B) {
	b.Run("ReadUncontended", func(b *testing.B) {
		var mu sync.RWMutex
		for b.Loop() {
			mu.RLock()
			mu.RUnlock()
		}
	})

	b.Run("ReadContended/4goroutines", func(b *testing.B) {
		var mu sync.RWMutex
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.RLock()
				mu.RUnlock()
			}
		})
	})

	b.Run("MixedReadWrite/90read10write", func(b *testing.B) {
		var mu sync.RWMutex
		var counter int64
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%10 == 0 {
					mu.Lock()
					counter++
					mu.Unlock()
				} else {
					mu.RLock()
					_ = counter
					mu.RUnlock()
				}
				i++
			}
		})
	})
}

// =============================================================================
// Benchmark: atomic operations (LSE provides hardware LDADD/SWPAL etc.)
// =============================================================================

func BenchmarkAtomicOps(b *testing.B) {
	b.Run("LoadInt32", func(b *testing.B) {
		var val int32
		for b.Loop() {
			atomic.LoadInt32(&val)
		}
	})

	b.Run("StoreInt32", func(b *testing.B) {
		var val int32
		for b.Loop() {
			atomic.StoreInt32(&val, 1)
		}
	})

	b.Run("AddInt64", func(b *testing.B) {
		var val int64
		for b.Loop() {
			atomic.AddInt64(&val, 1)
		}
	})

	b.Run("CompareAndSwapInt32", func(b *testing.B) {
		var val int32
		for b.Loop() {
			atomic.CompareAndSwapInt32(&val, 0, 1)
			atomic.CompareAndSwapInt32(&val, 1, 0)
		}
	})

	b.Run("AddInt64Contended/4goroutines", func(b *testing.B) {
		var val int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				atomic.AddInt64(&val, 1)
			}
		})
	})
}
