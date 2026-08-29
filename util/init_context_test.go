// Copyright (C) 2026 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/udr/context"
	"github.com/omec-project/udr/factory"
)

const (
	testVersion      = "1.0.0"
	testDescription  = "UDR test config"
	testScheme       = "https"
	testRegisterIPv4 = "127.0.0.1"
	testNrfUri       = "https://127.0.0.1:29510"
)

func newBaseConfig() factory.Config {
	return factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				Scheme:       testScheme,
				RegisterIPv4: testRegisterIPv4,
				Port:         8080,
			},
			NrfUri: testNrfUri,
		},
	}
}

func TestInitUdrContext_BasicFields(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = newBaseConfig()
	ctx := &context.UDRContext{}

	InitUdrContext(ctx)

	if ctx.NfId == "" {
		t.Fatal("NfId should be set")
	}
	if _, err := uuid.Parse(ctx.NfId); err != nil {
		t.Fatalf("NfId should be a valid UUID: %v", err)
	}
}

func TestInitUdrContext_DefaultValues(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: nil,
		},
	}
	ctx := &context.UDRContext{}

	InitUdrContext(ctx)

	if ctx.RegisterIPv4 != factory.UDR_DEFAULT_IPV4 {
		t.Errorf("RegisterIPv4 = %q, want %q", ctx.RegisterIPv4, factory.UDR_DEFAULT_IPV4)
	}
	if ctx.SBIPort != factory.UDR_DEFAULT_PORT_INT {
		t.Errorf("SBIPort = %d, want %d", ctx.SBIPort, factory.UDR_DEFAULT_PORT_INT)
	}
}

func TestInitUdrContext_SbiConfig(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				Scheme:       testScheme,
				RegisterIPv4: "10.0.0.1",
				Port:         9090,
			},
			NrfUri: testNrfUri,
		},
	}
	ctx := &context.UDRContext{}

	InitUdrContext(ctx)

	if ctx.UriScheme != models.UriScheme(testScheme) {
		t.Errorf("UriScheme = %q, want %q", ctx.UriScheme, testScheme)
	}
	if ctx.RegisterIPv4 != "10.0.0.1" {
		t.Errorf("RegisterIPv4 = %q, want %q", ctx.RegisterIPv4, "10.0.0.1")
	}
	if ctx.SBIPort != 9090 {
		t.Errorf("SBIPort = %d, want %d", ctx.SBIPort, 9090)
	}
}

func TestInitUdrContext_SbiConfig_EmptyRegisterIPv4(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				Scheme:       "http",
				RegisterIPv4: "",
				Port:         0,
			},
			NrfUri: testNrfUri,
		},
	}
	ctx := &context.UDRContext{}

	InitUdrContext(ctx)

	if ctx.RegisterIPv4 != factory.UDR_DEFAULT_IPV4 {
		t.Errorf("RegisterIPv4 = %q, want %q (default)", ctx.RegisterIPv4, factory.UDR_DEFAULT_IPV4)
	}
	if ctx.SBIPort != factory.UDR_DEFAULT_PORT_INT {
		t.Errorf("SBIPort = %d, want %d (default)", ctx.SBIPort, factory.UDR_DEFAULT_PORT_INT)
	}
}

func TestInitUdrContext_TLSConfig(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				Scheme:       testScheme,
				RegisterIPv4: testRegisterIPv4,
				Port:         8080,
				Tls: &factory.Tls{
					Key: "/path/to/key.pem",
					Pem: "/path/to/cert.pem",
				},
			},
			NrfUri: testNrfUri,
		},
	}
	ctx := &context.UDRContext{}

	InitUdrContext(ctx)

	if ctx.Key != "/path/to/key.pem" {
		t.Errorf("Key = %q, want %q", ctx.Key, "/path/to/key.pem")
	}
	if ctx.PEM != "/path/to/cert.pem" {
		t.Errorf("PEM = %q, want %q", ctx.PEM, "/path/to/cert.pem")
	}
}

func TestInitUdrContext_TLSConfig_EmptyValues(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				Scheme:       testScheme,
				RegisterIPv4: testRegisterIPv4,
				Port:         8080,
				Tls: &factory.Tls{
					Key: "",
					Pem: "",
				},
			},
			NrfUri: testNrfUri,
		},
	}
	ctx := &context.UDRContext{
		Key: "existing-key",
		PEM: "existing-pem",
	}

	InitUdrContext(ctx)

	if ctx.Key != "existing-key" {
		t.Errorf("Key = %q, want %q (should not be overridden when empty)", ctx.Key, "existing-key")
	}
	if ctx.PEM != "existing-pem" {
		t.Errorf("PEM = %q, want %q (should not be overridden when empty)", ctx.PEM, "existing-pem")
	}
}

func TestInitUdrContext_NrfUri_Set(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	expectedNrfUri := "https://10.0.0.2:29510"
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				Scheme:       testScheme,
				RegisterIPv4: testRegisterIPv4,
				Port:         8080,
			},
			NrfUri: expectedNrfUri,
		},
	}
	ctx := &context.UDRContext{}

	InitUdrContext(ctx)

	if ctx.NrfUri != expectedNrfUri {
		t.Errorf("NrfUri = %q, want %q", ctx.NrfUri, expectedNrfUri)
	}
}

func TestInitUdrContext_NrfUri_Empty_UsesDefault(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				Scheme:       testScheme,
				RegisterIPv4: testRegisterIPv4,
				Port:         8080,
			},
			NrfUri: "",
		},
	}
	ctx := &context.UDRContext{}

	InitUdrContext(ctx)

	expectedNrfUri := fmt.Sprintf("%s://%s:%d", ctx.UriScheme, testRegisterIPv4, 29510)
	if ctx.NrfUri != expectedNrfUri {
		t.Errorf("NrfUri = %q, want %q (default)", ctx.NrfUri, expectedNrfUri)
	}
}

func TestInitUdrContext_NilTLS(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi: &factory.Sbi{
				Scheme:       testScheme,
				RegisterIPv4: testRegisterIPv4,
				Port:         8080,
				Tls:          nil,
			},
			NrfUri: testNrfUri,
		},
	}
	ctx := &context.UDRContext{}

	// should not panic
	InitUdrContext(ctx)
}

func TestInitUdrContext_NilSbi(t *testing.T) {
	origUdrConfig := factory.UdrConfig
	t.Cleanup(func() { factory.UdrConfig = origUdrConfig })
	factory.UdrConfig = factory.Config{
		Info: &factory.Info{
			Version:     testVersion,
			Description: testDescription,
		},
		Configuration: &factory.Configuration{
			Sbi:    nil,
			NrfUri: testNrfUri,
		},
	}
	ctx := &context.UDRContext{}

	// should not panic
	InitUdrContext(ctx)

	if ctx.RegisterIPv4 != factory.UDR_DEFAULT_IPV4 {
		t.Errorf("RegisterIPv4 = %q, want %q (default)", ctx.RegisterIPv4, factory.UDR_DEFAULT_IPV4)
	}
	if ctx.SBIPort != factory.UDR_DEFAULT_PORT_INT {
		t.Errorf("SBIPort = %d, want %d (default)", ctx.SBIPort, factory.UDR_DEFAULT_PORT_INT)
	}
}
