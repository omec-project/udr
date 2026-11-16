// Copyright (C) 2026 Intel Corporation
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package factory

import (
	"testing"

	"github.com/omec-project/util/nfconfig"
)

func TestWebuiUrl(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
		want       string
	}{
		{
			name:       "default webui URL",
			configFile: "udr_config.yaml",
			want:       nfconfig.DefaultWebuiUri,
		},
		{
			name:       "custom webui URL",
			configFile: "udr_config_with_custom_webui_url.yaml",
			want:       "http://myspecialwebui:9872",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore original config
			origUdrConfig := UdrConfig
			t.Cleanup(func() { UdrConfig = origUdrConfig })

			if err := InitConfigFactory(tt.configFile); err != nil {
				t.Errorf("error in InitConfigFactory: %v", err)
			}

			got := UdrConfig.Configuration.WebuiUri
			if got != tt.want {
				t.Errorf("The webui URL is not correct. got = %q, want = %q", got, tt.want)
			}
		})
	}
}
