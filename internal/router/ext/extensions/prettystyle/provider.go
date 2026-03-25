package prettystyle

import (
	"fmt"

	"policycheck/internal/router/capabilities"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// Provider renders high-contrast text and structured CLI tables using go-pretty.
type Provider struct{}

// StyleText translates canonical roles from cli.go into go-pretty text styling.
// It enforces standardized 9-character gutters for perfect vertical alignment.
func (p *Provider) StyleText(kind string, input string) (string, error) {
	// Golden Rule: Standardized Gutters. Every gutter is exactly 9 characters long
	// to guarantee perfect vertical alignment when stacked in the terminal.
	switch kind {
	case "", capabilities.TextKindPlain:
		return input, nil
	case capabilities.TextKindHeader:
		return text.Colors{text.FgHiMagenta, text.Bold}.Sprint(input), nil
	case capabilities.TextKindDebug:
		gutter := text.Colors{text.FgHiMagenta, text.Bold}.Sprint("[DEBUG ] ")
		return gutter + text.Colors{text.FgHiWhite}.Sprint(input), nil
	case capabilities.TextKindInfo:
		gutter := text.Colors{text.FgHiCyan, text.Bold}.Sprint("[ INFO ] ")
		return gutter + text.Colors{text.FgHiWhite}.Sprint(input), nil
	case capabilities.TextKindSuccess:
		gutter := text.Colors{text.FgHiGreen, text.Bold}.Sprint("[  OK  ] ")
		return gutter + text.Colors{text.FgHiWhite}.Sprint(input), nil
	case capabilities.TextKindWarning:
		gutter := text.Colors{text.FgHiYellow, text.Bold}.Sprint("[ WARN ] ")
		return gutter + text.Colors{text.FgHiWhite}.Sprint(input), nil
	case capabilities.TextKindError:
		gutter := text.Colors{text.FgHiRed, text.Bold}.Sprint("[ ERR  ] ")
		return gutter + text.Colors{text.FgHiWhite}.Sprint(input), nil
	case capabilities.TextKindFatal:
		// Fatal gets an inverted/aggressive background to scream "CRASH"
		gutter := text.Colors{text.BgHiRed, text.FgHiWhite, text.Bold}.Sprint("[FATAL ]") + " "
		return gutter + text.Colors{text.FgHiRed, text.Bold}.Sprint(input), nil
	case capabilities.TextKindMuted:
		// Golden Rule: No "Invisible Grey". We use standard white to de-emphasize.
		return text.Colors{text.FgWhite}.Sprint(input), nil
	default:
		return "", fmt.Errorf("style text: unsupported kind %q", kind)
	}
}

// StyleTable translates header/row slices into a formatted ASCII table.
// It uses go-pretty exclusively for layout math (soft-wrapping, auto-merging).
func (p *Provider) StyleTable(headers []string, rows [][]string) (string, error) {
	if len(headers) == 0 {
		return "", fmt.Errorf("style table: headers must not be empty")
	}

	t := table.NewWriter()

	// 1. Setup Headers
	headerRow := make(table.Row, len(headers))
	for index, header := range headers {
		styledHeader, err := p.StyleText(capabilities.TextKindHeader, header)
		if err != nil {
			return "", fmt.Errorf("style table header %d: %w", index, err)
		}
		headerRow[index] = styledHeader
	}
	t.AppendHeader(headerRow)

	// 2. Setup Rows
	for rowIndex, row := range rows {
		if len(row) != len(headers) {
			return "", fmt.Errorf(
				"style table: row %d width %d does not match header width %d",
				rowIndex,
				len(row),
				len(headers),
			)
		}

		tableRow := make(table.Row, len(row))
		for columnIndex, cell := range row {
			// Payload text uses High Intensity White for maximum contrast
			tableRow[columnIndex] = text.Colors{text.FgHiWhite}.Sprint(cell)
		}
		t.AppendRow(tableRow)
	}

	// 3. Apply Golden Rule Table Layout
	t.SetStyle(table.StyleRounded)
	t.Style().Format.Header = text.FormatDefault
	t.Style().Options.SeparateRows = true // Keeps complex/wrapped data readable

	// 4. Configure Auto-Merging and Soft-Wrapping
	// go-pretty column configurations use 1-based indexing.
	colConfigs := make([]table.ColumnConfig, 0, len(headers))

	// Column 1: Vertical Merging (Ideal for grouped categories or file paths)
	colConfigs = append(colConfigs, table.ColumnConfig{
		Number:    1,
		AutoMerge: true,
	})

	// Column 2 through N: Soft-wrapping at 60 characters
	for i := 2; i <= len(headers); i++ {
		colConfigs = append(colConfigs, table.ColumnConfig{
			Number:   i,
			WidthMax: 60,
		})
	}

	t.SetColumnConfigs(colConfigs)

	// Render the final string
	return t.Render(), nil
}
