package config

import "encoding/json"

type StorageType string

const StorageTypeFile StorageType = "file"
const StorageTypeMemory StorageType = "memory"
const StorageTypeS3 StorageType = "s3"
const StorageTypePostgres StorageType = "postgres"
const StorageTypeSqlite StorageType = "sqlite"

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

func (s *Store) GetCertificateStoreType() StorageType {
	return getValidatedStorageType(s.Type, []StorageType{StorageTypeFile, StorageTypeS3, StorageTypeMemory})
}

// Config represents the configuration for Certmatic.
type Config struct {
	DomainStores []Store `json:"domain_stores,omitempty"`
	CertStores   []Store `json:"cert_stores,omitempty"`
}

type FileStorageConfig struct {
	RootPath string `json:"root_path,omitempty"`
}

type S3StorageConfig struct {
	BucketName string `json:"bucket_name,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	AccessKey  string `json:"access_key,omitempty"`
	SecretKey  string `json:"secret_key,omitempty"`

	// Endpoint is optional; if not set, the default AWS endpoint for the region will be used.
	Endpoint string `json:"endpoint,omitempty"`
	// Region is ignored iif Endpoint is set
	Region string `json:"region,omitempty"`
}

type InmemoryStorageConfig struct {
	// No specific config needed for in-memory storage
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

func AsFileStorageConfig(rawConfig map[string]any) (*FileStorageConfig, error) {
	return getTypedStorageConfig[FileStorageConfig](rawConfig)
}

func AsS3StorageConfig(rawConfig map[string]any) (*S3StorageConfig, error) {
	return getTypedStorageConfig[S3StorageConfig](rawConfig)
}

func AsInmemoryStorageConfig(rawConfig map[string]any) (*InmemoryStorageConfig, error) {
	return getTypedStorageConfig[InmemoryStorageConfig](rawConfig)
}
