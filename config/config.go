package config

import "encoding/json"

// Config contains the assemble chat server params
type Config struct {
	SMTP struct {
		SslHostPort string `json:"sslhostport"` // smtp.google.com:465
		Username    string `json:"username"`    // myuser@gmail.com
		Password    string `json:"password"`    // ****
		From        string `json:"from"`        //usualy same as username
		Enabled     bool   `json:"enabled"`     //false
	} `json:"smtp"`

	AdminPass     string `json:"adminpass"`     //whatever
	Host          string `json:"host"`          //localhost
	Bind          string `json:"bind"`          //:443
	DefaultMaxExp string `json:"defaultmaxexp"` //48h
	DefaultMinExp string `json:"defaultminexp"` //30s
	UserTimeout   string `json:"usertimeout"`   //300s
	LastAlertWait string `json:"lastalertwait"` //30m - prevents sending too many alerts to offline users
}

// DefaultConfig returns a default configuration struct
func DefaultConfig() (*Config, error) {
	var defaults = `{
		"smtp": {
            "sslhostport": "smtp.google.com:465",
            "username": "myuser@gmail.com",
            "password": "****",
            "from": "myuser@gmail.com",
            "enabled": false
        },
        "adminpass": "PASS",
        "host": "localhost",
        "bind": ":443",
        "defaultmaxexp": "48h",
        "defaultminexp": "30s",
        "usertimeout": "300s",
		"lastalertwait": "30m"
	}`

	// Unmarshal the default json string into an interface.
	var defaultConfig map[string]interface{}
	if err := json.Unmarshal([]byte(defaults), &defaultConfig); err != nil {
		return nil, err
	}

	// Create a new variable of the Config struct and populate it with the default values.
	config := &Config{}
	if err := json.Unmarshal([]byte(defaults), &config); err != nil {
		return nil, err
	}
	return config, nil
}

// LoadConfig takes an existing Config and a string of json and overwrites present params
func LoadConfig(config *Config, custom string) (*Config, error) {
	if err := json.Unmarshal([]byte(custom), &config); err != nil {
		return nil, err
	}
	return config, nil
}
