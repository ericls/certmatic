package config

import "encoding/json"

type StorageType string

const (
	StorageTypeMemory   StorageType = "memory"
	StorageTypePostgres StorageType = "postgres"
	StorageTypeSqlite   StorageType = "sqlite"
)

type Store struct {
	Type   string                 `json:"type,omitempty"`
	Config map[string]interface{} `json:"config,omitempty"`
}

func getValidatedStorageType(strValue string, allowedTypes []StorageType) StorageType {
	for _, allowed := range allowedTypes {
		if StorageType(strValue) == allowed {
			return allowed
		}
	}
	panic("invalid storage type: " + strValue)
}

func (s *Store) GetDomainStoreType() StorageType {
	return getValidatedStorageType(s.Type, []StorageType{StorageTypeMemory, StorageTypePostgres, StorageTypeSqlite})
}

// Config represents the configuration for Certmatic.
type Config struct {
	DomainStores []Store `json:"domain_stores,omitempty"`
}

type InmemoryStorageConfig struct {
	// No specific config needed for in-memory storage
}

type PostgresStorageConfig struct {
	ConnectionString string `json:"connection_string,omitempty"`
}

type SqliteStorageConfig struct {
	FilePath string `json:"file_path,omitempty"`
}

func getTypedStorageConfig[T any](rawConfig map[string]any) (*T, error) {
	if rawConfig == nil {
		return nil, nil
	}
	var typedConfig T
	bytes, err := json.Marshal(rawConfig)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &typedConfig)
	if err != nil {
		return nil, err
	}
	return &typedConfig, nil
}

func AsInmemoryStorageConfig(rawConfig map[string]any) (*InmemoryStorageConfig, error) {
	return getTypedStorageConfig[InmemoryStorageConfig](rawConfig)
}

func AsPostgresStorageConfig(rawConfig map[string]any) (*PostgresStorageConfig, error) {
	return getTypedStorageConfig[PostgresStorageConfig](rawConfig)
}

func AsSqliteStorageConfig(rawConfig map[string]any) (*SqliteStorageConfig, error) {
	return getTypedStorageConfig[SqliteStorageConfig](rawConfig)
}
