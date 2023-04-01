package environment

import (
	"encoding/json"
	environment2 "github.com/GSH-LAN/Unwindia_common/src/go/environment"
	"github.com/GSH-LAN/Unwindia_common/src/go/logger"
	"github.com/GSH-LAN/Unwindia_common/src/go/messagebroker"
	pulsarClient "github.com/apache/pulsar-client-go/pulsar"
	envLoader "github.com/caarlos0/env/v6"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/ksuid"
	"net/url"
	"runtime"
	"time"
)

var (
	env *Environment
)

// environment holds all the environment variables with primitive types
type environment struct {
	environment2.BaseEnvironment

	ServiceUid string

	JobsProcessInterval time.Duration `env:"JOBS_PROCESS_INTERVAL" envDefault:"10s"`
	UseMatchServiceId   bool          `env:"USE_MATCHSERVICE_ID" envDefault:false`

	PteroApplicationApiURL   url.URL       `env:"PTERODACTYL_APPLICATION_API_URL,notEmpty"`
	PteroApplicationApiToken string        `env:"PTERODACTYL_APPLICATION_API_TOKEN,notEmpty"`
	PteroClientApiURL        url.URL       `env:"PTERODACTYL_CLIENT_API_URL,notEmpty"`
	PteroClientApiToken      string        `env:"PTERODACTYL_CLIENT_API_TOKEN,notEmpty"`
	PteroFetchInterval       time.Duration `env:"PTERODACTYL_FETCH_INTERVAL" envDefault:"15s"`

	RconRetries int `env:"RCON_RETRIES" envDefault:"10" envDescription:"How often to retry"`
}

// Environment holds all environment configuration with more advanced typing and validation
type Environment struct {
	environment
	PulsarAuth pulsarClient.Authentication
}

// Load initialized the environment variables
func load() *Environment {
	e := environment{}
	if err := envLoader.Parse(&e); err != nil {
		log.Panic().Err(err)
	}

	if err := logger.SetLogLevel(e.LogLevel); err != nil {
		log.Panic().Err(err)
	}

	if e.WorkerCount <= 0 {
		e.WorkerCount = runtime.NumCPU() + e.WorkerCount
	}

	if e.ServiceUid == "" {
		e.ServiceUid = ksuid.New().String()
	}

	var pulsarAuthParams = make(map[string]string)
	if e.PulsarAuthParams != "" {
		if err := json.Unmarshal([]byte(e.PulsarAuthParams), &pulsarAuthParams); err != nil {
			log.Panic().Err(err)
		}
	}

	var mbpulsarAuth messagebroker.PulsarAuth
	if err := mbpulsarAuth.Unmarshal(e.PulsarAuth); err != nil {
		log.Panic().Err(err)
	}

	var pulsarAuth pulsarClient.Authentication

	switch mbpulsarAuth {
	case messagebroker.AUTH_OAUTH2:
		pulsarAuth = pulsarClient.NewAuthenticationOAuth2(pulsarAuthParams)
	}

	e2 := Environment{
		environment: e,
		PulsarAuth:  pulsarAuth,
	}

	log.Info().Interface("environemt", e2).Msgf("Loaded Environment")

	return &e2
}

func Get() *Environment {
	if env == nil {
		env = load()
	}

	return env
}
