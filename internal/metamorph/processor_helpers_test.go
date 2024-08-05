package metamorph

import (
	"fmt"
	"testing"
)

func BenchmarkUnorderedEqual(b *testing.B) {
	bcs := []int{1, 10, 100, 1000, 10000}

	for _, n := range bcs {
		b.Run(fmt.Sprintf("txIDs-%d", n), func(b *testing.B) {
			txIDs := make([]string, n)
			txIDs2 := make([]string, n)

			for i := 0; i < n; i++ {
				txIDs[i] = fmt.Sprintf("tx-%d", i)
				txIDs2[i] = fmt.Sprintf("tx-%d", i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = unorderedEqual(txIDs, txIDs2)
			}
		})
	}
}

func BenchmarkCmpDiff(b *testing.B) {
	bcs := []int{1, 10, 100, 1000, 10000}

	for _, n := range bcs {
		b.Run(fmt.Sprintf("txIDs-%d", n), func(b *testing.B) {
			txIDs := make([]string, n)
			txIDs2 := make([]string, n)

			for i := 0; i < n; i++ {
				txIDs[i] = fmt.Sprintf("tx-%d", i)
				txIDs2[i] = fmt.Sprintf("tx-%d", i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = cmpDiff(txIDs, txIDs2)
			}
		})
	}
}
