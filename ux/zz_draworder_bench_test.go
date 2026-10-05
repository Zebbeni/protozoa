package ux

import (
	"math/rand"
	"testing"

	"github.com/Zebbeni/protozoa/organism"
)

func benchInfos(n int) map[int]*organism.Info {
	infos := make(map[int]*organism.Info, n)
	for id := 1; id <= n; id++ {
		st := organism.StatusChemoSuccess
		if rand.Intn(200) == 0 {
			st = organism.StatusDying
		}
		infos[id] = &organism.Info{ID: id, Status: st, BornThisCycle: rand.Intn(50) == 0}
	}
	return infos
}

// BenchmarkDrawOrder at a realistic population. One map range per layer,
// no sort: ~110us a frame at 5,000 organisms, against ~440us for a version
// that also ordered within each layer.
func BenchmarkDrawOrder(b *testing.B) {
	infos := benchInfos(5000)
	var buf []*organism.Info
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = organismDrawOrder(infos, buf[:0])
	}
}
