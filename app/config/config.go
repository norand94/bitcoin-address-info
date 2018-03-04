package config

type M struct {
	Mongo Mongo `yaml:"mongo"`
	Redis Redis `yaml:"redis"`

	Port           string `yaml:"port"`
	LoaderRoutines int    `yaml:"loader_routines"`
}

type Redis struct {
	DbNum     string `yaml:"db_num"`
	Address   string `yaml:"address"`
	ExpireSec string `yaml:"expire_sec"`
	Password  string `yaml:"password"`
}

type Mongo struct {
	DialUrl string `yaml:"dial_url"`
	DbName  string `yaml:"db_name"`
}
