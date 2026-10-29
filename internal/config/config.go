// Package config 读取默认值、TOML 和环境变量。
package config

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	HTTPAddr      string       `toml:"http_addr"`
	DataDir       string       `toml:"data_dir"`
	Log           LogConfig    `toml:"log"`
	Jobs          JobConfig    `toml:"jobs"`
	Auth          AuthConfig   `toml:"auth"`
	Upload        UploadConfig `toml:"upload"`
	AdminUser     string       `toml:"-"`
	AdminPassword string       `toml:"-"`
}

type LogConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

type JobConfig struct {
	Workers      int      `toml:"workers"`
	PollInterval Duration `toml:"poll_interval"`
}

type AuthConfig struct {
	SessionTTL   Duration `toml:"session_ttl"`
	CookieSecure bool     `toml:"cookie_secure"`
}

type UploadConfig struct {
	MaxBytes int64 `toml:"max_bytes"`
}

// Duration 让 TOML 可以用 "1s" 这种写法。
type Duration time.Duration

func (d Duration) Std() time.Duration { return time.Duration(d) }

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func Defaults() Config {
	return Config{
		HTTPAddr: ":8080",
		DataDir:  "data",
		Log:      LogConfig{Level: "info", Format: "text"},
		Jobs:     JobConfig{Workers: 1, PollInterval: Duration(time.Second)},
		Auth:     AuthConfig{SessionTTL: Duration(7 * 24 * time.Hour)},
		Upload:   UploadConfig{MaxBytes: 2 << 30},
	}
}

func (c Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "hearth.db")
}

func (c Config) OriginalsDir() string {
	return filepath.Join(c.DataDir, "originals")
}

func (c Config) CacheDir() string {
	return filepath.Join(c.DataDir, "cache")
}

// Load 按默认值、配置文件、环境变量的顺序合并。
// 未指定 -config 时，若当前目录存在 hearth.toml 则使用它。
func Load(args []string, getenv func(string) string) (Config, error) {
	fs := flag.NewFlagSet("hearth", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "path to hearth.toml")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	cfg := Defaults()
	path := *configPath
	if path == "" {
		path = getenv("HEARTH_CONFIG")
	}
	if path == "" {
		if _, err := os.Stat("hearth.toml"); err == nil {
			path = "hearth.toml"
		}
	}
	if path != "" {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return Config{}, fmt.Errorf("read config %s: %w", path, err)
		}
	}
	if err := applyEnv(&cfg, getenv); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config, getenv func(string) string) error {
	if v := getenv("HEARTH_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if v := getenv("HEARTH_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := getenv("HEARTH_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := getenv("HEARTH_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := getenv("HEARTH_JOBS_WORKERS"); v != "" {
		var n int
		if _, err := fmt.Sscan(v, &n); err != nil {
			return fmt.Errorf("HEARTH_JOBS_WORKERS: %w", err)
		}
		cfg.Jobs.Workers = n
	}
	if v := getenv("HEARTH_JOBS_POLL_INTERVAL"); v != "" {
		if err := cfg.Jobs.PollInterval.UnmarshalText([]byte(v)); err != nil {
			return fmt.Errorf("HEARTH_JOBS_POLL_INTERVAL: %w", err)
		}
	}
	if v := getenv("HEARTH_SESSION_TTL"); v != "" {
		if err := cfg.Auth.SessionTTL.UnmarshalText([]byte(v)); err != nil {
			return fmt.Errorf("HEARTH_SESSION_TTL: %w", err)
		}
	}
	if v := getenv("HEARTH_COOKIE_SECURE"); v != "" {
		cfg.Auth.CookieSecure = v == "1" || strings.EqualFold(v, "true")
	}
	if v := getenv("HEARTH_UPLOAD_MAX_BYTES"); v != "" {
		var n int64
		if _, err := fmt.Sscan(v, &n); err != nil {
			return fmt.Errorf("HEARTH_UPLOAD_MAX_BYTES: %w", err)
		}
		cfg.Upload.MaxBytes = n
	}
	cfg.AdminUser = getenv("HEARTH_ADMIN_USER")
	cfg.AdminPassword = getenv("HEARTH_ADMIN_PASSWORD")
	return nil
}

func (c Config) validate() error {
	if c.HTTPAddr == "" {
		return fmt.Errorf("http_addr is empty")
	}
	if c.DataDir == "" {
		return fmt.Errorf("data_dir is empty")
	}
	if c.Jobs.Workers < 1 {
		return fmt.Errorf("jobs.workers must be >= 1")
	}
	if c.Jobs.PollInterval.Std() <= 0 {
		return fmt.Errorf("jobs.poll_interval must be > 0")
	}
	if c.Auth.SessionTTL.Std() <= 0 {
		return fmt.Errorf("auth.session_ttl must be > 0")
	}
	if c.Upload.MaxBytes <= 0 {
		return fmt.Errorf("upload.max_bytes must be > 0")
	}
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level %q is invalid", c.Log.Level)
	}
	switch strings.ToLower(c.Log.Format) {
	case "text", "json":
	default:
		return fmt.Errorf("log.format %q is invalid", c.Log.Format)
	}
	return nil
}

func NewLogger(cfg LogConfig) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(cfg.Format, "json") {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}
