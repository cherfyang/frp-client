package dl

import (
	"sync"
	"sync/atomic"
	"time"
)

type ProgressInfo struct {
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
	Speed      int64   `json:"speed"`
	Done       bool    `json:"done"`
	Error      string  `json:"error"`
}

type Progress struct {
	downloaded atomic.Int64
	total      atomic.Int64
	done       atomic.Bool
	mu         sync.Mutex
	err        string
	startedAt  time.Time
}

func newProgress() *Progress {
	return &Progress{startedAt: time.Now()}
}

func (p *Progress) Add(n int64)      { p.downloaded.Add(n) }
func (p *Progress) SetTotal(n int64) { p.total.Store(n) }

func (p *Progress) SetDone() { p.done.Store(true) }

func (p *Progress) SetError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
	p.done.Store(true)
}

func (p *Progress) Snapshot() ProgressInfo {
	downloaded := p.downloaded.Load()
	total := p.total.Load()
	pct := float64(0)
	if total > 0 {
		pct = float64(downloaded) / float64(total) * 100
		if pct > 100 {
			pct = 100
		}
	}
	speed := int64(0)
	elapsed := time.Since(p.startedAt).Seconds()
	if elapsed > 0.1 {
		speed = int64(float64(downloaded) / elapsed)
	}
	p.mu.Lock()
	err := p.err
	p.mu.Unlock()
	return ProgressInfo{
		Downloaded: downloaded,
		Total:      total,
		Percentage: pct,
		Speed:      speed,
		Done:       p.done.Load(),
		Error:      err,
	}
}
