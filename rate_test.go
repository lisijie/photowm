package main

import (
    "testing"
    "time"
)

func TestRateLimit_Take(t *testing.T) {
    rl := NewRateLimit(100)
    last := rl.Take()
    for i := 0; i < 100; i++ {
        tt := rl.Take()
        if tt.Sub(last) / time.Millisecond != 10 {
            t.Fail()
        }
        last = tt
    }
}