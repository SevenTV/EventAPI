package client

import (
	"github.com/seventv/common/utils"
)

type Cache interface {
	AddDispatch(h uint32) bool
	HasDispatch(h uint32) bool
	ExpireDispatch(h uint32)
}

type cacheInst struct {
	dispatch  utils.Set[uint32]
	ordered   []uint32
	idx       int
	oldestIdx int
}

func NewCache() Cache {
	return &cacheInst{
		dispatch: make(utils.Set[uint32], 4*1024),
		ordered:  make([]uint32, 2*1024),
	}
}

func (c *cacheInst) AddDispatch(h uint32) bool {
	had := c.dispatch.Has(h)

	if !had {
		idx := c.idx % len(c.ordered)
		c.ordered[idx] = h
		c.idx = idx + 1
	}

	for len(c.dispatch) >= len(c.ordered) {
		oldest := c.ordered[c.oldestIdx]
		c.oldestIdx = (c.oldestIdx + 1) % len(c.ordered)
		c.dispatch.Delete(oldest)
	}

	c.dispatch.Add(h)

	return !had
}

func (c *cacheInst) ExpireDispatch(h uint32) {
	c.dispatch.Delete(h)
}

func (c *cacheInst) HasDispatch(h uint32) bool {
	return c.dispatch.Has(h)
}
