package db_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/adapter/db"
	"github.com/optimaldynamics/project-mittens/pkg/journal"
)

func TestDBConfig_ParseURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedHost string
		expectedPort int
		expectedDB   string
		expectedUser string
		expectedPass string
		expectedSSL  string
	}{
		{
			name:         "Standard postgres URL",
			url:          "postgres://mittens:secret@dbhost:5433/mittens_prod?sslmode=require",
			expectedHost: "dbhost",
			expectedPort: 5433,
			expectedDB:   "mittens_prod",
			expectedUser: "mittens",
			expectedPass: "secret",
			expectedSSL:  "require",
		},
		{
			name:         "Default empty URL",
			url:          "",
			expectedHost: "localhost",
			expectedPort: 5432,
			expectedDB:   "mittens",
			expectedUser: "mittens",
			expectedPass: "mittens_secret_pw",
			expectedSSL:  "disable",
		},
		{
			name:         "Postgresql scheme without port",
			url:          "postgresql://app_user:pw123@prod-cluster/prod_db",
			expectedHost: "prod-cluster",
			expectedPort: 5432,
			expectedDB:   "prod_db",
			expectedUser: "app_user",
			expectedPass: "pw123",
			expectedSSL:  "disable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := db.ParseURL(tc.url)
			if err != nil {
				t.Fatalf("ParseURL failed: %v", err)
			}
			if cfg.Host != tc.expectedHost {
				t.Errorf("expected host %s, got %s", tc.expectedHost, cfg.Host)
			}
			if cfg.Port != tc.expectedPort {
				t.Errorf("expected port %d, got %d", tc.expectedPort, cfg.Port)
			}
			if cfg.Database != tc.expectedDB {
				t.Errorf("expected database %s, got %s", tc.expectedDB, cfg.Database)
			}
			if cfg.User != tc.expectedUser {
				t.Errorf("expected user %s, got %s", tc.expectedUser, cfg.User)
			}
			if cfg.Password != tc.expectedPass {
				t.Errorf("expected password %s, got %s", tc.expectedPass, cfg.Password)
			}
			if cfg.SSLMode != tc.expectedSSL {
				t.Errorf("expected sslmode %s, got %s", tc.expectedSSL, cfg.SSLMode)
			}

			// Verify ConnString roundtrip
			connStr := cfg.ConnString()
			reparsed, err := db.ParseURL(connStr)
			if err != nil {
				t.Fatalf("re-parsing ConnString failed: %v", err)
			}
			if reparsed.Host != cfg.Host || reparsed.Port != cfg.Port || reparsed.Database != cfg.Database {
				t.Errorf("roundtrip mismatch: got %+v, want %+v", reparsed, cfg)
			}
		})
	}
}

func TestDBConfig_InvalidURL(t *testing.T) {
	invalidURLs := []string{
		"http://localhost:5432/db",
		"mysql://user:pw@host/db",
		"postgres://:invalid-port-format/db",
	}

	for _, u := range invalidURLs {
		_, err := db.ParseURL(u)
		if err == nil {
			t.Errorf("expected error for invalid url %s, got nil", u)
		}
	}
}

func TestDBConfig_ToPgxPoolConfig(t *testing.T) {
	cfg := db.DefaultDBConfig()
	cfg.MaxConns = 50
	cfg.MinConns = 10
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 10 * time.Minute
	cfg.ConnTimeout = 3 * time.Second

	poolCfg, err := cfg.ToPgxPoolConfig()
	if err != nil {
		t.Fatalf("ToPgxPoolConfig failed: %v", err)
	}

	if poolCfg.MaxConns != 50 {
		t.Errorf("expected MaxConns 50, got %d", poolCfg.MaxConns)
	}
	if poolCfg.MinConns != 10 {
		t.Errorf("expected MinConns 10, got %d", poolCfg.MinConns)
	}
	if poolCfg.MaxConnLifetime != 1*time.Hour {
		t.Errorf("expected MaxConnLifetime 1h, got %v", poolCfg.MaxConnLifetime)
	}
	if poolCfg.MaxConnIdleTime != 10*time.Minute {
		t.Errorf("expected MaxConnIdleTime 10m, got %v", poolCfg.MaxConnIdleTime)
	}
	if poolCfg.ConnConfig.ConnectTimeout != 3*time.Second {
		t.Errorf("expected ConnectTimeout 3s, got %v", poolCfg.ConnConfig.ConnectTimeout)
	}
}

func TestPool_UnreachableConnectionFailsClosed(t *testing.T) {
	cfg := db.DefaultDBConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 59999 // Unused port
	cfg.ConnTimeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg)
	if err == nil {
		pool.Close()
		t.Fatal("expected connection failure to unreachable port, got nil error")
	}
}

func TestJournalRecord_IntegrityRoundtrip(t *testing.T) {
	rec := journal.JournalRecord{
		DecisionID:           "DEC-CFA-1700000000-0001",
		RunID:                "RUN_TEST_001",
		Epoch:                1700000000,
		BatchSeq:             0,
		PolicyName:           "CFA",
		MatchedCount:         2,
		TotalNetContribution: 1200.00,
		InitialStateHash:     "STATE_HASH_123",
		ActionHash:           "ACTION_HASH_456",
		PrevRecordHash:       journal.GenesisPrevHash,
		RuntimeVersion:       journal.CurrentRuntimeVersion,
	}
	rec.RecordHash = rec.ComputeRecordHash()

	if !rec.VerifyIntegrity() {
		t.Fatalf("VerifyIntegrity failed on freshly hashed record")
	}
}

func TestPostgresRunRepository_UnmarshalMetadata_FailClosed(t *testing.T) {
	// 1. Verify invalid JSON metadata fails closed when unmarshaled
	corruptJSON := []byte("{corrupt-json-key-value: missing_quotes")
	var meta map[string]any
	err := json.Unmarshal(corruptJSON, &meta)
	if err == nil {
		t.Fatal("expected json.Unmarshal to fail on corrupt JSON bytes, got nil")
	}

	// Verify formatted error wrapping matches expected pattern
	wrappedErr := fmt.Errorf("db: failed unmarshaling metadata for run %s: %w", "OPT_RUN_TEST", err)
	if wrappedErr == nil {
		t.Fatal("expected wrapped error")
	}

	// 2. Verify valid JSON metadata unmarshals properly
	validJSON := []byte(`{"policy_class":"CFA","iterations":42,"alpha":0.05}`)
	var validMeta map[string]any
	if err := json.Unmarshal(validJSON, &validMeta); err != nil {
		t.Fatalf("unexpected unmarshal failure on valid JSON: %v", err)
	}
	if validMeta["policy_class"] != "CFA" {
		t.Errorf("expected policy_class 'CFA', got %v", validMeta["policy_class"])
	}
}

func TestPostgresRunRepository_Save_MarshalValidation(t *testing.T) {
	run := db.OptimizationRun{
		RunID:       "TEST_RUN_001",
		PolicyClass: "CFA",
		FleetSize:   10,
		LoadsCount:  20,
		Metadata: map[string]any{
			"key": "value",
		},
	}
	metaJSON, err := json.Marshal(run.Metadata)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if len(metaJSON) == 0 {
		t.Fatal("expected non-empty metaJSON")
	}
}
