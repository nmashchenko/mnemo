package deps

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"mnemo/clog"
	"mnemo/services/ws"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/InVisionApp/go-health"
	"mnemo/config"
)

const (
	DefaultHealthCheckIntervalSecs = 1
)

type customCheck struct{}

type Dependencies struct {
	// Services
	WebsocketManager *ws.Manager

	Health health.IHealth

	// Global, shared shutdown context - all services and backends listen to
	// this context to know when to shutdown.
	ShutdownCtx context.Context

	// ShutdownCancel is the cancel function for the global shutdown context
	ShutdownCancel context.CancelFunc

	// Channel written to by publisher when it's done shutting down; read by
	// shutdown handler in main(). We need this so that we can tell the shutdown
	// handler when it is safe to exit.
	PublisherShutdownDoneCh chan struct{}

	Config *config.Config

	// Log is the main, shared logger (you should use this for all logging)
	Log clog.ICustomLog

	// ZapLog is the zap logger (you shouldn't need this outside of deps)
	ZapLog *zap.Logger

	// ZapCore can be used to generate a brand-new logger (you shouldn't need this very often)
	ZapCore zapcore.Core
}

func New(cfg *config.Config) (*Dependencies, error) {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Dependencies{
		ShutdownCtx:             ctx,
		ShutdownCancel:          cancel,
		PublisherShutdownDoneCh: make(chan struct{}),
		Config:                  cfg,
	}

	if err := d.setupLogging(); err != nil {
		return nil, errors.Wrap(err, "unable to setup logging")
	}

	// Pretty print config in dev mode
	if d.Config.LogConfig == "dev" {
		d.LogConfig()
	}

	if err := d.setupHealth(); err != nil {
		return nil, errors.Wrap(err, "unable to setup health")
	}

	if err := d.Health.Start(); err != nil {
		return nil, errors.Wrap(err, "unable to start health runner")
	}

	if err := d.setupServices(cfg); err != nil {
		return nil, errors.Wrap(err, "unable to setup services")
	}

	return d, nil
}

// If using New Relic, setupLogging() should be called _after_ setupNewRelic()
func (d *Dependencies) setupLogging() error {
	var core zapcore.Core

	if d.Config.LogConfig == "dev" {
		zc := zap.NewDevelopmentConfig()
		zc.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

		core = zapcore.NewCore(zapcore.NewConsoleEncoder(zc.EncoderConfig),
			zapcore.AddSync(os.Stdout),
			zap.DebugLevel,
		)
	} else {
		core = zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(os.Stdout),
			zap.InfoLevel,
		)
	}

	// Save the actual loggers
	d.ZapLog = zap.New(core)
	d.ZapCore = core

	// Create a new primary logger that will be passed to everyone
	d.Log = clog.New(d.ZapLog, zap.String("env", d.Config.EnvName))

	d.Log.Debug("Logging initialized")

	return nil
}

func (d *Dependencies) setupHealth() error {
	logger := d.Log.With(zap.String("method", "setupHealth"))
	logger.Debug("Setting up health")

	gohealth := health.New()
	gohealth.DisableLogging()

	cc := &customCheck{}

	err := gohealth.AddChecks([]*health.Config{
		{
			Name:     "health-check",
			Checker:  cc,
			Interval: time.Duration(DefaultHealthCheckIntervalSecs) * time.Second,
			Fatal:    true,
		},
	})

	d.Health = gohealth

	if err != nil {
		return err
	}

	return nil
}

func (d *Dependencies) setupServices(cfg *config.Config) error {
	logger := d.Log.With(zap.String("method", "setupServices"))
	logger.Debug("Setting up services")

	logger.Debug("Setting up hub service")

	manager := ws.NewManager()
	d.WebsocketManager = manager

	return nil
}

// Status satisfies the go-health.ICheckable interface
func (c *customCheck) Status() (interface{}, error) {
	if false {
		return nil, errors.New("something major just broke")
	}

	// You can return additional information pertaining to the check as long
	// as it can be JSON marshalled
	return map[string]int{}, nil
}

// LogConfig pretty prints the config to the log
func (d *Dependencies) LogConfig() {
	d.ZapLog.Info("Config")

	longestKey := 0

	for k, _ := range d.Config.GetMap() {
		if len(k) > longestKey {
			longestKey = len(k)
		}
	}

	maxPadding := longestKey + 3
	totalKeys := len(d.Config.GetMap())
	index := 0
	prefix := "├─"

	for k, v := range d.Config.GetMap() {
		index++

		if index == totalKeys {
			prefix = "└─"
		}

		padding := maxPadding - len(k)

		line := fmt.Sprintf("%s %s %s %-"+strconv.Itoa(len(k))+"v", prefix, k, strings.Repeat(" ", padding), v)
		d.ZapLog.Debug(line)
	}
}
