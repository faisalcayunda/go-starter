package core

import (
	"strconv"
	"sync/atomic"
)

// newIDGen mengembalikan generator id pesanan sederhana berbasis counter atomik —
// murni stdlib, tanpa dependency eksternal (mis. UUID). Cukup untuk contoh & test;
// ganti dengan UUID/ULID saat butuh keunikan terdistribusi.
func newIDGen() func() string {
	var counter atomic.Int64
	return func() string {
		return "ord-" + strconv.FormatInt(counter.Add(1), 10)
	}
}
