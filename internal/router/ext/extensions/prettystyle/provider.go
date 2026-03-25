package prettystyle

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"policycheck/internal/router/capabilities"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

const layoutSplitGap = "    "

// Provider renders high-contrast text, structured tables, and basic layouts.
type Provider struct{}

// StyleText translates canonical roles from cli.go into go-pretty text styling.
func (p *Provider) StyleText(kind string, input string) (string, error) {
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
		gutter := text.Colors{text.BgHiRed, text.FgHiWhite, text.Bold}.Sprint("[FATAL ]") + " "
		return gutter + text.Colors{text.FgHiRed, text.Bold}.Sprint(input), nil
	case capabilities.TextKindMuted:
		return text.Colors{text.FgWhite}.Sprint(input), nil
	default:
		return "", fmt.Errorf("style text: unsupported kind %q", kind)
	}
}

// StyleTable translates header/row slices into a formatted table string.
func (p *Provider) StyleTable(kind string, headers []string, rows [][]string) (string, error) {
	if len(headers) == 0 {
		return "", fmt.Errorf("style table: headers must not be empty")
	}

	tableWriter := table.NewWriter()
	headerRow := make(table.Row, len(headers))
	for index, header := range headers {
		styledHeader, err := p.StyleText(capabilities.TextKindHeader, header)
		if err != nil {
			return "", fmt.Errorf("style table header %d: %w", index, err)
		}

		headerRow[index] = styledHeader
	}
	tableWriter.AppendHeader(headerRow)

	for rowIndex, row := range rows {
		tableRow, err := p.mapTableRow(kind, rowIndex, row, len(headers))
		if err != nil {
			return "", err
		}

		tableWriter.AppendRow(tableRow)
	}

	p.applyTableKind(tableWriter, kind, len(headers))
	return tableWriter.Render(), nil
}

func (p *Provider) mapTableRow(kind string, rowIndex int, row []string, headerWidth int) (table.Row, error) {
	if len(row) != headerWidth {
		return nil, fmt.Errorf(
			"style table: row %d width %d does not match header width %d",
			rowIndex,
			len(row),
			headerWidth,
		)
	}

	tableRow := make(table.Row, len(row))
	for columnIndex, cell := range row {
		if kind == capabilities.TableKindMuted {
			mutedCell, err := p.StyleText(capabilities.TextKindMuted, cell)
			if err != nil {
				return nil, fmt.Errorf("style muted table cell [%d,%d]: %w", rowIndex, columnIndex, err)
			}

			tableRow[columnIndex] = mutedCell
		} else {
			tableRow[columnIndex] = cell
		}
	}

	return tableRow, nil
}

// StyleLayout renders semantic layout containers for CLI output.
func (p *Provider) StyleLayout(kind string, title string, content ...string) (string, error) {
	switch kind {
	case capabilities.LayoutKindPanel:
		return p.renderPanelLayout(title, content), nil
	case capabilities.LayoutKindSplit:
		return p.renderSplitLayout(content), nil
	case capabilities.LayoutKindGutter:
		return p.renderGutterLayout(title, content), nil
	default:
		return "", fmt.Errorf("style layout: unsupported kind %q", kind)
	}
}

func (p *Provider) applyTableKind(tableWriter table.Writer, kind string, columnCount int) {
	style := table.StyleRounded
	separateRows := true

	switch kind {
	case capabilities.TableKindCompact, capabilities.TableKindMuted:
		style = table.StyleLight
		separateRows = false
	case capabilities.TableKindMerged:
		tableWriter.SetColumnConfigs([]table.ColumnConfig{{
			Number:    1,
			AutoMerge: true,
		}})
	}

	tableWriter.SetStyle(style)
	tableWriter.Style().Options.SeparateRows = separateRows

	tableWriter.Style().Format.Header = text.FormatDefault

	if kind == capabilities.TableKindCompact {
		configs := make([]table.ColumnConfig, 0, columnCount)
		for index := 1; index <= columnCount; index++ {
			configs = append(configs, table.ColumnConfig{
				Number:   index,
				WidthMax: 40,
			})
		}
		tableWriter.SetColumnConfigs(configs)
	}
}

func (p *Provider) renderPanelLayout(title string, content []string) string {
	lines := p.layoutLines(content)
	width := utf8.RuneCountInString(title)
	for _, line := range lines {
		if lineWidth := utf8.RuneCountInString(line); lineWidth > width {
			width = lineWidth
		}
	}

	if width == 0 {
		width = 1
	}

	topBorder := "╭" + strings.Repeat("─", width+2) + "╮"
	if title != "" {
		topBorder = fmt.Sprintf("╭─ %s %s╮", title, strings.Repeat("─", max(width-utf8.RuneCountInString(title), 0)))
	}

	body := make([]string, 0, len(lines)+2)
	body = append(body, topBorder)
	for _, line := range lines {
		body = append(body, fmt.Sprintf("│ %-*s │", width, line))
	}
	body = append(body, "╰"+strings.Repeat("─", width+2)+"╯")

	return strings.Join(body, "\n")
}

func (p *Provider) renderSplitLayout(content []string) string {
	linesByBlock := make([][]string, 0, len(content))
	widths := make([]int, 0, len(content))

	for _, block := range content {
		lines := strings.Split(block, "\n")
		linesByBlock = append(linesByBlock, lines)
		width := 0
		for _, line := range lines {
			if lineWidth := utf8.RuneCountInString(line); lineWidth > width {
				width = lineWidth
			}
		}
		widths = append(widths, width)
	}

	maxLines := 0
	for _, lines := range linesByBlock {
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}

	rendered := make([]string, 0, maxLines)
	for lineIndex := 0; lineIndex < maxLines; lineIndex++ {
		parts := make([]string, 0, len(linesByBlock))
		for blockIndex, lines := range linesByBlock {
			value := ""
			if lineIndex < len(lines) {
				value = lines[lineIndex]
			}
			parts = append(parts, fmt.Sprintf("%-*s", widths[blockIndex], value))
		}
		rendered = append(rendered, strings.Join(parts, layoutSplitGap))
	}

	return strings.Join(rendered, "\n")
}

func (p *Provider) renderGutterLayout(title string, content []string) string {
	lines := p.layoutLines(content)
	gutter := fmt.Sprintf("%-9.9s", title)
	rendered := make([]string, 0, len(lines))
	for index, line := range lines {
		prefix := gutter
		if index > 0 {
			prefix = strings.Repeat(" ", len(gutter))
		}
		rendered = append(rendered, prefix+line)
	}

	return strings.Join(rendered, "\n")
}

func (p *Provider) layoutLines(content []string) []string {
	if len(content) == 0 {
		return []string{""}
	}

	lines := make([]string, 0, len(content))
	for _, block := range content {
		lines = append(lines, strings.Split(block, "\n")...)
	}

	return lines
}
