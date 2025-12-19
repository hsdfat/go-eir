package repository

import (
	"sort"

	"github.com/hsdfat8/eir/models"
)

type ImeiRepository interface {
	Lookup(startRange string) (*models.ImeiInfo, bool)
	Save(info *models.ImeiInfo) error
	ListAll() []models.ImeiInfo
	Clear()
}

type InMemoryImeiRepo struct {
	data       map[string]*models.ImeiInfo
	sortedKeys []string
}

func NewInMemoryImeiRepo() *InMemoryImeiRepo {
	return &InMemoryImeiRepo{
		data: make(map[string]*models.ImeiInfo),
	}
}

func (r *InMemoryImeiRepo) Lookup(start string) (*models.ImeiInfo, bool) {
	v, ok := r.data[start]
	return v, ok
}

func (r *InMemoryImeiRepo) Save(info *models.ImeiInfo) error {
	r.data[info.StartIMEI] = info

	exists := false
	for _, s := range r.sortedKeys {
		if s == info.StartIMEI {
			exists = true
			break
		}
	}
	if !exists {
		r.sortedKeys = append(r.sortedKeys, info.StartIMEI)
	}

	sort.Strings(r.sortedKeys)

	return nil
}
