package main

import (
	"encoding/json"
	"os"
)

type config struct {
	Log struct {
		Level string `json:"level"`
		File  string `json:"file"`
	} `json:"log"`
	StratumServer struct {
		Address         string   `json:"address"`
		Port            int      `json:"port"`
		BackupInterval  string   `json:"backup_interval"`
		OmitAgentStatus []string `json:"omit_agent_status"`
	} `json:"stratum_server"`
	APIServer struct {
		Address  string `json:"address"`
		Port     int    `json:"port"`
		AuthUser string `json:"auth_user"`
		AuthPass string `json:"auth_pass"`
	} `json:"api_server"`
	Storage struct {
		Address  string `json:"address"`
		Port     int    `json:"port"`
		Db       int    `json:"db"`
		Password string `json:"password"`
	} `json:"storage"`
	Node struct {
		Address     string `json:"address"`
		APIPort     int    `json:"api_port"`
		StratumPort int    `json:"stratum_port"`
		AuthUser    string `json:"auth_user"`
		AuthPass    string `json:"auth_pass"`
		Diff        int    `json:"diff"`
		BlockTime   int    `json:"block_time"`
	} `json:"node"`
	Wallet struct {
		Address         string `json:"address"`
		OwnerAPIVersion string `json:"owner_api_version"`
		OwnerAPIPort    int    `json:"owner_api_port"`
		AuthUser        string `json:"auth_user"`
		AuthPass        string `json:"auth_pass"`
	} `json:"wallet"`
	Payer struct {
		Time string  `json:"time"`
		Fee  float64 `json:"fee"`
	} `json:"payer"`
}

func parseConfig() *config {
	f, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}

	var conf config
	dec := json.NewDecoder(f)
	err = dec.Decode(&conf)
	if err != nil {
		panic(err)
	}

	return &conf
}
