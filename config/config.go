package config

import (
	"github.com/koding/multiconfig"
)

func init() {
	InitConfig()
}

type (
	// Config global config
	Config struct {
		Debug   bool
		Action  string
		Db      Db
		Fetcher Fetcher
		Packer  Packer
		Sender  Sender
	}

	// Db config for db
	Db struct {
		User   string
		Passwd string
		Host   string
		Port   string
		Table  string
	}

	// Fetcher config for fetcher
	Fetcher struct {
		Timeout     int
		Host        string
		Concurrency int
	}

	// Packer config for packer
	Packer struct {
		BlockSize int `toml:"block_size"`
	}

	// Sender config for sender
	Sender struct {
		Timeout    int
		PrivateKey string `toml:"private_key"`
		MchID      string `toml:"mch_id"`
		LedgerID   string `toml:"ledger_id"`
		ChainID    string `toml:"chain_id"`
	}
)

// Conf global config
var Conf Config

// InitConfig inits config from file
func InitConfig() {
	m := multiconfig.NewWithPath("config/config.toml")
	m.MustLoad(&Conf)
	//fmt.Printf("%#v\n", Conf)
}
