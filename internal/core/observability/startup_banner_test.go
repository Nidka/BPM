package observability

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Nidka/BPM-service/internal/core/config"
	"github.com/Nidka/BPM-service/internal/core/contracts"
)

var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestStartupBannerShowsSafeRuntimeInformationOnce(t *testing.T) {
	var output bytes.Buffer
	banner := NewStartupBanner(config.AppConfig{
		Name:        "bpm-api",
		Environment: config.Development,
		Version:     "dev",
	}, &output)
	info := contracts.StartupInfo{
		Address:       "0.0.0.0:8080",
		Port:          8080,
		APIURL:        "http://localhost:8080/api/v1",
		HealthURL:     "http://localhost:8080/health",
		ReadinessURL:  "http://localhost:8080/ready",
		DatabaseReady: true,
	}

	banner.ReportStartup(info)
	banner.ReportStartup(info)

	plainOutput := ansiSequence.ReplaceAllString(output.String(), "")
	for _, expected := range []string{
		"BPM API  ·  SERVIDOR EN EJECUCIÓN",
		"● API LISTA",
		"● POSTGRESQL CONECTADO",
		"Ambiente: desarrollo",
		"Puerto: 8080",
		"http://localhost:8080/api/v1",
		"By: Kadin, Evelyn y Piero",
		"Listo para recibir solicitudes",
	} {
		if !strings.Contains(plainOutput, expected) {
			t.Fatalf("expected banner to contain %q:\n%s", expected, plainOutput)
		}
	}
	if strings.Count(plainOutput, "Listo para recibir solicitudes") != 1 {
		t.Fatalf("expected banner to render once:\n%s", plainOutput)
	}
	if !strings.Contains(output.String(), "\x1b[") {
		t.Fatal("expected development banner to contain ANSI colors")
	}
	for _, line := range strings.Split(plainOutput, "\n") {
		if strings.HasPrefix(line, "╭") || strings.HasPrefix(line, "├") ||
			strings.HasPrefix(line, "│") || strings.HasPrefix(line, "╰") {
			if width := utf8.RuneCountInString(line); width != 114 {
				t.Fatalf("expected a 114-column banner line, got %d: %q", width, line)
			}
		}
	}
}

func TestStartupBannerIsDisabledInProduction(t *testing.T) {
	var output bytes.Buffer
	banner := NewStartupBanner(config.AppConfig{
		Environment: config.Production,
	}, &output)

	banner.ReportStartup(contracts.StartupInfo{Port: 8080})

	if output.Len() != 0 {
		t.Fatalf("production must not render terminal banner: %q", output.String())
	}
}
