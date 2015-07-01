package backends

import (
    "container/ring"
    "sync"
)

type Backends interface {
	Choose() Backend
	Len() int
	Add(string, string)
	Remove(string)
}

type backend struct {
	name string
	host string
}

func (b *backend) Host() string {
    return b.host
}

func (b *backend) Name() string {
	return b.name
}
type Backend interface {
	Host() string
	Name() string
}


type RoundRobin struct {
    r   *ring.Ring
    l   sync.RWMutex
}

func NewRoundRobin(strs map[string]string) Backends {
    r := ring.New(len(strs))
    for key, s := range strs {
        r.Value = &backend{key,s}
        r = r.Next()
    }
    return &RoundRobin{r: r}
}

func (rr *RoundRobin) Len() int {
    rr.l.RLock()
    defer rr.l.RUnlock()
    return rr.r.Len()
}

func (rr *RoundRobin) Choose() Backend {
    rr.l.Lock()
    defer rr.l.Unlock()
    if rr.r == nil {
        return nil
    }
    n := rr.r.Value.(*backend)
    rr.r = rr.r.Next()
    return n
}
func (rr *RoundRobin) Add(name string, host string) {
    rr.l.Lock()
    defer rr.l.Unlock()
    nr := &ring.Ring{Value: &backend{name, host}}
    if rr.r == nil {
        rr.r = nr
    } else {
        rr.r = rr.r.Link(nr).Next()
    }
}

func (rr *RoundRobin) Remove(s string) {
    rr.l.Lock()
    defer rr.l.Unlock()
    r := rr.r
    if rr.r.Len() == 1 {
        rr.r = ring.New(0)
        return
    }

    for i := rr.r.Len(); i > 0; i-- {
        r = r.Next()
        ba := r.Value.(*backend)
        if s == ba.Host() {
            rr.r = r.Unlink(1)
            return
        }
    }
}
