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

func (r *InMemoryImeiRepo) ListAll() []models.ImeiInfo {
	result := make([]models.ImeiInfo, 0, len(r.data))
	for _, key := range r.sortedKeys {
		if info, ok := r.data[key]; ok {
			result = append(result, *info)
		}
	}
	return result
}

func (r *InMemoryImeiRepo) Clear() {
	r.data = make(map[string]*models.ImeiInfo)
	r.sortedKeys = []string{}
}
