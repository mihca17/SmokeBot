package config

type Config struct {
	BotToken    string
	DBType      string `yaml:"db_type"`
	DBPath      string `yaml:"db_path"`
	DBTableName string `yaml:"db_table_name"`
	Port        int    `yaml:"port"`
	LogFile     string `yaml:"log_file"`
}

func DefaultConfig() Config {
	return Config{
		BotToken: "8304451768:AAEyfAUAWL2jNgDQI-MfKVHObe71BBtAJ98",
		DBPath:   "./data.db",
		LogFile:  "logs.log",
	}
}
