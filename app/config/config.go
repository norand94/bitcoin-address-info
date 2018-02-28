package config

type M struct {
	MongoDialUrl string `yaml:"mongo_dial_url"`
	MongoDbName string `yaml:"mongo_db_name"`

	Port string `yaml:"port"`
}
