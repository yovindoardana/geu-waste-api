package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantError bool
	}{
		{
			name: "valid config",
			cfg: Config{
				AppEnv:           "development",
				AppPort:          8080,
				DBHost:           "localhost",
				DBPort:           5432,
				DBName:           "geu_waste",
				DBUser:           "postgres",
				DBPassword:       "postgres",
				DBSSLMode:        "disable",
				UploadDir:        "./uploads/payment-proofs",
				DBConnectTimeout: 5 * time.Second,
				ShutdownTimeout:  10 * time.Second,
			},
			wantError: false,
		},
		{
			name: "invalid app env",
			cfg: Config{
				AppEnv:           "invalid_env",
				AppPort:          8080,
				DBHost:           "localhost",
				DBPort:           5432,
				DBName:           "geu_waste",
				DBUser:           "postgres",
				DBPassword:       "postgres",
				DBSSLMode:        "disable",
				UploadDir:        "./uploads/payment-proofs",
				DBConnectTimeout: 5 * time.Second,
				ShutdownTimeout:  10 * time.Second,
			},
			wantError: true,
		},
		{
			name: "invalid port",
			cfg: Config{
				AppEnv:           "development",
				AppPort:          0,
				DBHost:           "localhost",
				DBPort:           5432,
				DBName:           "geu_waste",
				DBUser:           "postgres",
				DBPassword:       "postgres",
				DBSSLMode:        "disable",
				UploadDir:        "./uploads/payment-proofs",
				DBConnectTimeout: 5 * time.Second,
				ShutdownTimeout:  10 * time.Second,
			},
			wantError: true,
		},
		{
			name: "missing db host",
			cfg: Config{
				AppEnv:           "development",
				AppPort:          8080,
				DBHost:           "",
				DBPort:           5432,
				DBName:           "geu_waste",
				DBUser:           "postgres",
				DBPassword:       "postgres",
				DBSSLMode:        "disable",
				UploadDir:        "./uploads/payment-proofs",
				DBConnectTimeout: 5 * time.Second,
				ShutdownTimeout:  10 * time.Second,
			},
			wantError: true,
		},
		{
			name: "missing db password",
			cfg: Config{
				AppEnv:           "development",
				AppPort:          8080,
				DBHost:           "localhost",
				DBPort:           5432,
				DBName:           "geu_waste",
				DBUser:           "postgres",
				DBPassword:       "",
				DBSSLMode:        "disable",
				UploadDir:        "./uploads/payment-proofs",
				DBConnectTimeout: 5 * time.Second,
				ShutdownTimeout:  10 * time.Second,
			},
			wantError: true,
		},
		{
			name: "invalid ssl mode",
			cfg: Config{
				AppEnv:           "development",
				AppPort:          8080,
				DBHost:           "localhost",
				DBPort:           5432,
				DBName:           "geu_waste",
				DBUser:           "postgres",
				DBPassword:       "postgres",
				DBSSLMode:        "prefer",
				UploadDir:        "./uploads/payment-proofs",
				DBConnectTimeout: 5 * time.Second,
				ShutdownTimeout:  10 * time.Second,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestDSNEncoding(t *testing.T) {
	cfg := Config{
		AppEnv:           "development",
		AppPort:          8080,
		DBHost:           "localhost",
		DBPort:           5432,
		DBName:           "geu_waste",
		DBUser:           "user@domain",
		DBPassword:       "p@ss:w/ord",
		DBSSLMode:        "disable",
		UploadDir:        "./uploads/payment-proofs",
		DBConnectTimeout: 5 * time.Second,
		ShutdownTimeout:  10 * time.Second,
	}

	dsn := cfg.DSN()
	if dsn == "" {
		t.Fatal("DSN should not be empty")
	}

	safe := cfg.SafeDSN()
	if safe == "" {
		t.Fatal("SafeDSN should not be empty")
	}
	if safe == dsn {
		t.Fatal("SafeDSN must mask the password")
	}
}

func TestLoadWithEnv(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_NAME", "geu_waste_test")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASSWORD", "secret")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_NAME")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASSWORD")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error = %v", err)
	}

	if cfg.AppEnv != "test" {
		t.Errorf("expected AppEnv 'test', got %q", cfg.AppEnv)
	}
	if cfg.DBName != "geu_waste_test" {
		t.Errorf("expected DBName 'geu_waste_test', got %q", cfg.DBName)
	}
}
