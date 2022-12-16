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
	dispatch utils.Set[uint32]
}

func NewCache() Cache {
	return &cacheInst{
		dispatch: make(utils.Set[uint32]),
	}
}

// AddDispatch implements Cache
func (c *cacheInst) AddDispatch(h uint32) bool {
	had := c.dispatch.Has(h)

	c.dispatch.Add(h)

	return !had
}

// ExpireDispatch implements Cache
func (c *cacheInst) ExpireDispatch(h uint32) {
	c.dispatch.Delete(h)
}

// HasDispatch implements Cache
func (c *cacheInst) HasDispatch(h uint32) bool {
	return c.dispatch.Has(h)
}
