package main

import (
    "time"
    "sync"
)

type RateLimit struct {
    sync.Mutex
    preRequest time.Duration
    last       time.Time
}

func NewRateLimit(rate int) *RateLimit {
    return &RateLimit{preRequest: time.Second / time.Duration(rate)}
}

func (r *RateLimit) Take() time.Time {
    r.Lock()
    defer r.Unlock()

    now := time.Now()

    if r.last.IsZero() {
        r.last = now
        return r.last
    }

    sleep := r.preRequest - now.Sub(r.last)

    if sleep > 0 {
        time.Sleep(sleep)
        r.last = now.Add(sleep)
    } else {
        r.last = now
    }
    return r.last
}