package observability

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/Nidka/BPM-service/internal/core/config"
	"github.com/Nidka/BPM-service/internal/core/contracts"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiGray   = "\033[90m"
)

var bpmLogo = []string{
	"██████╗ ██████╗ ███╗   ███╗",
	"██╔══██╗██╔══██╗████╗ ████║",
	"██████╔╝██████╔╝██╔████╔██║",
	"██╔══██╗██╔═══╝ ██║╚██╔╝██║",
	"██████╔╝██║     ██║ ╚═╝ ██║",
	"╚═════╝ ╚═╝     ╚═╝     ╚═╝",
}

// StartupBanner renders the local development summary once the application is
// actually ready. Production keeps using structured JSON logs only.
type StartupBanner struct {
	app     config.AppConfig
	output  io.Writer
	enabled bool
	once    sync.Once
}

func NewStartupBanner(app config.AppConfig, output io.Writer) *StartupBanner {
	return &StartupBanner{
		app:     app,
		output:  output,
		enabled: !app.IsProductionLike(),
	}
}

func DefaultStartupBanner(app config.AppConfig) *StartupBanner {
	return NewStartupBanner(app, os.Stdout)
}

func (banner *StartupBanner) ReportStartup(info contracts.StartupInfo) {
	if banner == nil || !banner.enabled || banner.output == nil {
		return
	}

	banner.once.Do(func() {
		banner.render(info)
	})
}

func (banner *StartupBanner) render(info contracts.StartupInfo) {
	const (
		contentWidth = 112
		logoWidth    = 34
	)
	border := strings.Repeat("─", contentWidth)
	divider := strings.Repeat("─", logoWidth)
	rightBorder := strings.Repeat("─", contentWidth-logoWidth-1)
	databaseStatus := "● POSTGRESQL NO DISPONIBLE"
	if info.DatabaseReady {
		databaseStatus = "● POSTGRESQL CONECTADO"
	}

	rightLines := []struct {
		text  string
		color string
	}{
		{fmt.Sprintf("  %s  ·  SERVIDOR EN EJECUCIÓN", displayName(banner.app.Name)), ansiBold + ansiCyan},
		{"  ● API LISTA     " + databaseStatus, ansiGreen},
		{fmt.Sprintf("  Ambiente: %s     Versión: %s", environmentName(banner.app.Environment), banner.app.Version), ansiReset},
		{fmt.Sprintf("  Dirección: %s     Puerto: %d", info.Address, info.Port), ansiYellow},
		{"  API:        " + info.APIURL, ansiCyan},
		{"  Salud:      " + info.HealthURL, ansiCyan},
		{"  Disponible: " + info.ReadinessURL, ansiCyan},
	}

	fmt.Fprintf(banner.output, "\n%s╭%s╮%s\n", ansiCyan, border, ansiReset)
	for index, right := range rightLines {
		logoLine := ""
		if index < len(bpmLogo) {
			logoLine = "  " + bpmLogo[index]
		}
		banner.splitRow(contentWidth, logoWidth, logoLine, right.text, right.color)
	}
	fmt.Fprintf(banner.output, "%s├%s┴%s┤%s\n", ansiCyan, divider, rightBorder, ansiReset)
	banner.footer(contentWidth, "By: Kadin, Evelyn y Piero", "● Listo para recibir solicitudes")
	fmt.Fprintf(banner.output, "%s╰%s╯%s\n\n", ansiCyan, border, ansiReset)
}

func environmentName(environment string) string {
	switch environment {
	case config.Development:
		return "desarrollo"
	case config.Test:
		return "pruebas"
	case config.Staging:
		return "preproducción"
	case config.Production:
		return "producción"
	default:
		return environment
	}
}

func displayName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", " "))
}

func (banner *StartupBanner) splitRow(
	contentWidth int,
	leftWidth int,
	left string,
	right string,
	rightColor string,
) {
	rightWidth := contentWidth - leftWidth - 1
	fmt.Fprintf(banner.output, "%s│%s%s%-*s%s%s│%s%s%-*s%s%s│%s\n",
		ansiCyan,
		ansiReset,
		ansiBold+ansiCyan,
		leftWidth,
		left,
		ansiReset,
		ansiCyan,
		ansiReset,
		rightColor,
		rightWidth,
		right,
		ansiReset,
		ansiCyan,
		ansiReset,
	)
}

func (banner *StartupBanner) footer(contentWidth int, authors, status string) {
	authors = "  " + authors
	status = status + "  "
	padding := contentWidth - len([]rune(authors)) - len([]rune(status))
	if padding < 1 {
		padding = 1
	}

	fmt.Fprintf(banner.output, "%s│%s%s%s%s%s%s%s%s│%s\n",
		ansiCyan,
		ansiReset,
		ansiGray,
		authors,
		ansiReset,
		strings.Repeat(" ", padding),
		ansiBold+ansiGreen,
		status,
		ansiReset+ansiCyan,
		ansiReset,
	)
}
