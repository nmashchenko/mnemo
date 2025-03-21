package config

import (
	"fmt"
	"reflect"

	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	EnvFile         = ".env"
	EnvConfigPrefix = "GO_MNEMO"
)

type Config struct {
	Version          kong.VersionFlag `help:"Show version and exit" short:"v" env:"-"`
	EnvName          string           `kong:"help='Environment name.',default='dev'"`
	ServiceName      string           `kong:"help='Service name.',default='go-mnemo'"`
	EnablePprof      bool             `kong:"help='Enable pprof endpoints (http://$apiListenAddress/debug).',default=false"`
	APIListenAddress string           `kong:"help='API listen address (serves health, metrics, version).',default=:8080"`
	LogConfig        string           `kong:"help='Logging config to use.',enum='dev,prod',default='dev'"`

	NumGeneratorWorkers int `kong:"help='Number of generator workers to run.',default=4"`

	KongContext *kong.Context `kong:"-"`
}

func New(version string) *Config {
	// Attempt to load .env - do not fail if it's not there. Only environment
	// that might have this is in local/dev; staging, prod should not have one.
	if err := godotenv.Load(EnvFile); err != nil {
		zap.L().Warn("unable to load dotenv file", zap.String("err", err.Error()))
	}

	cfg := &Config{}
	cfg.KongContext = kong.Parse(
		cfg,
		kong.Name("go-mnemo"),
		kong.Description("Golang service"),
		kong.DefaultEnvars(EnvConfigPrefix),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		}),
		kong.Vars{
			"version": version,
		},
	)

	return cfg
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("Config cannot be nil")
	}

	return nil
}

// GetMap generates a map of field:value pairs for all fields in Config struct
func (c *Config) GetMap() map[string]string {
	fields := make(map[string]string)

	val := reflect.ValueOf(c)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	t := val.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := val.Field(i)
		fields[field.Name] = fmt.Sprintf("%v", value)
	}

	return fields
}
