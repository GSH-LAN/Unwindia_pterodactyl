package config

import "github.com/parkervcp/crocgodyl"

type UnwindiaPteroConfig struct {
	Configs map[string]GamerServerConfigTemplate // configs is a map which contains the game name as key with belonging template
}

type GamerServerConfigTemplate struct {
	UserId               int                    `json:"userId"`
	LocationId           int                    `json:"locationId"`
	NestId               int                    `json:"nestId"`
	ServerNamePrefix     string                 `json:"serverNamePrefix"`
	ServerNameGOTVPrefix string                 `json:"serverNameGOTVPrefix"`
	DefaultStartup       string                 `json:"defaultStartup"`
	DefaultDockerImage   string                 `json:"defaultDockerImage"`
	EggId                int                    `json:"eggId"`
	Limits               crocgodyl.Limits       `json:"limits"`      // Limits which will be set for new servers.
	ForceLimits          bool                   `json:"forceLimits"` // if true, server which does not meet Limits settings will be deleted. If false, existing suspended servers will be reused, no matter of matching Limits
	Environment          map[string]interface{} `json:"environment"` // Environment settings for a game. Can be values or matching properties of a gameserver match thingy object in go-template like format // TODO: set correct object description
	TvSlots              int                    `json:"tvSlots"`
}
