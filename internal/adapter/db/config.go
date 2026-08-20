package db

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DBConfig encapsulates all parameters required to establish and manage a PostgreSQL connection pool.
//
// In accordance with Inviolate 0 (Explicit Configuration), all connection parameters
// must be supplied explicitly with no ambient/global state.
type DBConfig struct {
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	Database        string        `json:"database"`
	User            string        `json:"user"`
	Password        string        `json:"password"`
	SSLMode         string        `json:"ssl_mode"`
	MaxConns        int32         `json:"max_conns"`
	MinConns        int32         `json:"min_conns"`
	MaxConnLifetime time.Duration `json:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `json:"max_conn_idle_time"`
	ConnTimeout     time.Duration `json:"conn_timeout"`
}

// DefaultDBConfig returns production-tuned PostgreSQL connection pool defaults.
func DefaultDBConfig() DBConfig {
	return DBConfig{
		Host:            "localhost",
		Port:            5432,
		Database:        "mittens",
		User:            "mittens",
		Password:        "mittens_secret_pw",
		SSLMode:         "disable",
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: 30 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
		ConnTimeout:     5 * time.Second,
	}
}

// ParseURL parses a postgres:// connection string into an explicit DBConfig.
func ParseURL(connStr string) (DBConfig, error) {
	cfg := DefaultDBConfig()
	if connStr == "" {
		return cfg, nil
	}

	u, err := url.Parse(connStr)
	if err != nil {
		return DBConfig{}, fmt.Errorf("db: malformed connection url: %w", err)
	}

	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return DBConfig{}, fmt.Errorf("db: invalid scheme %q, expected postgres or postgresql", u.Scheme)
	}

	if u.Hostname() != "" {
		cfg.Host = u.Hostname()
	}
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			cfg.Port = p
		}
	}
	if u.Path != "" {
		cfg.Database = strings.TrimPrefix(u.Path, "/")
	}
	if u.User != nil {
		cfg.User = u.User.Username()
		if pw, set := u.User.Password(); set {
			cfg.Password = pw
		}
	}

	q := u.Query()
	if ssl := q.Get("sslmode"); ssl != "" {
		cfg.SSLMode = ssl
	}

	return cfg, nil
}

// ConnString formats the DBConfig as a standard PostgreSQL connection string.
func (c DBConfig) ConnString() string {
	u := url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.Database,
	}
	if c.User != "" {
		if c.Password != "" {
			u.User = url.UserPassword(c.User, c.Password)
		} else {
			u.User = url.User(c.User)
		}
	}
	q := u.Query()
	if c.SSLMode != "" {
		q.Set("sslmode", c.SSLMode)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// ToPgxPoolConfig converts DBConfig to a pgxpool.Config with explicit pool limits.
func (c DBConfig) ToPgxPoolConfig() (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(c.ConnString())
	if err != nil {
		return nil, fmt.Errorf("db: failed parsing pool config: %w", err)
	}

	if c.MaxConns > 0 {
		poolCfg.MaxConns = c.MaxConns
	}
	if c.MinConns > 0 {
		poolCfg.MinConns = c.MinConns
	}
	if c.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = c.MaxConnLifetime
	}
	if c.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = c.MaxConnIdleTime
	}
	if c.ConnTimeout > 0 {
		poolCfg.ConnConfig.ConnectTimeout = c.ConnTimeout
	}

	return poolCfg, nil
}
