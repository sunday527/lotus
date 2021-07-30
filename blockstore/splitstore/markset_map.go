package splitstore

import (
	"sync"

	cid "github.com/ipfs/go-cid"
)

type MapMarkSetEnv struct{}

var _ MarkSetEnv = (*MapMarkSetEnv)(nil)

type MapMarkSet struct {
	mx  sync.RWMutex
	set map[string]struct{}

	ts bool
}

var _ MarkSet = (*MapMarkSet)(nil)

type MapMarkSetVisitor struct {
	set map[string]struct{}
}

var _ MarkSetVisitor = (*MapMarkSetVisitor)(nil)

func NewMapMarkSetEnv() (*MapMarkSetEnv, error) {
	return &MapMarkSetEnv{}, nil
}

func (e *MapMarkSetEnv) Create(name string, sizeHint int64) (MarkSet, error) {
	return &MapMarkSet{
		set: make(map[string]struct{}, sizeHint),
	}, nil
}

func (e *MapMarkSetEnv) CreateVisitor(name string, sizeHint int64) (MarkSetVisitor, error) {
	return &MapMarkSetVisitor{
		set: make(map[string]struct{}, sizeHint),
	}, nil
}

func (e *MapMarkSetEnv) Close() error {
	return nil
}

func (s *MapMarkSet) Mark(cid cid.Cid) error {
	if s.ts {
		s.mx.Lock()
		defer s.mx.Unlock()
	}

	if s.set == nil {
		return errMarkSetClosed
	}

	s.set[string(cid.Hash())] = struct{}{}
	return nil
}

func (s *MapMarkSet) Has(cid cid.Cid) (bool, error) {
	if s.ts {
		s.mx.RLock()
		defer s.mx.RUnlock()
	}

	if s.set == nil {
		return false, errMarkSetClosed
	}

	_, ok := s.set[string(cid.Hash())]
	return ok, nil
}

func (s *MapMarkSet) Close() error {
	if s.ts {
		s.mx.Lock()
		defer s.mx.Unlock()
	}
	s.set = nil
	return nil
}

func (s *MapMarkSet) SetConcurrent() {
	s.ts = true
}

func (v *MapMarkSetVisitor) Visit(c cid.Cid) (bool, error) {
	if v.set == nil {
		return false, errMarkSetClosed
	}

	key := string(c.Hash())
	if _, ok := v.set[key]; ok {
		return false, nil
	}

	v.set[key] = struct{}{}
	return true, nil
}

func (v *MapMarkSetVisitor) Close() error {
	v.set = nil
	return nil
}
