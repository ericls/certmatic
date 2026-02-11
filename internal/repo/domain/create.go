package domain

import (
	"github.com/ericls/certmatic/internal/config"
	domain "github.com/ericls/certmatic/pkg/domain"
)

func NewDomainStoreFromConfig(conf config.Store) (domain.DomainRepo, error) {
	storeType := conf.GetDomainStoreType()
	switch storeType {
	case config.StorageTypeMemory:
		_, err := config.AsInmemoryStorageConfig(conf.Config)
		if err != nil {
			return nil, err
		}
		return NewInMemoryDomainRepo("inmemory"), nil
	case config.StorageTypePostgres:
		_, err := config.AsPostgresStorageConfig(conf.Config)
		if err != nil {
			return nil, err
		}
		panic("Not implement")
	case config.StorageTypeSqlite:
		_, err := config.AsSqliteStorageConfig(conf.Config)
		if err != nil {
			return nil, err
		}
		panic("Not implement")
	default:
		panic("unsupported storage type: " + storeType)
	}
}
