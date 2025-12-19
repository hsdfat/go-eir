package repository

import (
	"sort"

	"github.com/hsdfat8/eir/models"
)

type TacRepository interface {
	Save(tac *models.TacInfo) error
	Lookup(key string) (*models.TacInfo, bool)
	Prev(key string) (*models.TacInfo, bool)
	Next(key string) (*models.TacInfo, bool)
	ListAll() []*models.TacInfo
}

type InMemoryTacRepo struct {
	data map[string]*models.TacInfo
}

func NewInMemoryTacRepo() *InMemoryTacRepo {
	return &InMemoryTacRepo{
		data: make(map[string]*models.TacInfo),
	}
}

func (r *InMemoryTacRepo) Save(tac *models.TacInfo) error {
	r.data[tac.KeyTac] = tac
	return nil
}

func (r *InMemoryTacRepo) Lookup(key string) (*models.TacInfo, bool) {
	t, ok := r.data[key]
	return t, ok
}

func (r *InMemoryTacRepo) Prev(key string) (*models.TacInfo, bool) {
	var prev *models.TacInfo
	for k, v := range r.data {
		if k < key {
			if prev == nil || k > prev.KeyTac {
				prev = v
			}
		}
	}
	if prev == nil {
		return nil, false
	}
	return prev, true
}

func (r *InMemoryTacRepo) Next(key string) (*models.TacInfo, bool) {
	var next *models.TacInfo
	for k, v := range r.data {
		if k > key {
			if next == nil || k < next.KeyTac {
				next = v
			}
		}
	}
	if next == nil {
		return nil, false
	}
	return next, true
}

func (r *InMemoryTacRepo) ListAll() []*models.TacInfo {
	keys := make([]string, 0, len(r.data))
	for k := range r.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]*models.TacInfo, 0, len(r.data))
	for _, k := range keys {
		result = append(result, r.data[k])
	}
	return result
}
