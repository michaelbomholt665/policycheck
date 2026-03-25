package charmcli

import (
	"fmt"
	"strings"

	"policycheck/internal/router/capabilities"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var (
	colorHiMagenta = lipgloss.Color("13")
	colorHiCyan    = lipgloss.Color("14")
	colorHiGreen   = lipgloss.Color("10")
	colorHiYellow  = lipgloss.Color("11")
	colorHiWhite   = lipgloss.Color("15")
	colorHiRed     = lipgloss.Color("9")

	titleStyle = lipgloss.NewStyle().
			Foreground(colorHiWhite).
			Background(colorHiMagenta).
			Bold(true).
			Padding(0, 1)
	descStyle = lipgloss.NewStyle().
			Foreground(colorHiCyan)
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorHiMagenta).
			Padding(1, 2)
)

const splitGap = "    "

// Provider renders semantic prompt flows and spatial layouts backed by Charm libraries.
type Provider struct {
	RunForm func(*huh.Form) error
}

// StylePrompt maps semantic prompt kinds to interactive huh forms.
func (p *Provider) StylePrompt(
	kind string,
	title string,
	description string,
	options []capabilities.Choice,
) (any, error) {
	theme := p.newTheme()

	switch kind {
	case capabilities.PromptKindSelect:
		return p.runSelect(title, description, options, theme)
	case capabilities.PromptKindToggle:
		return p.runToggle(title, description, options, theme)
	case capabilities.PromptKindConfirm:
		return p.runConfirm(title, description, theme)
	case capabilities.PromptKindInput:
		return p.runInput(title, description, theme)
	default:
		return nil, fmt.Errorf("style prompt: unsupported kind %q", kind)
	}
}

// StyleText translates semantic roles into lipgloss-styled text.
func (p *Provider) StyleText(kind string, input string) (string, error) {
	switch kind {
	case "", capabilities.TextKindPlain:
		return input, nil
	case capabilities.TextKindHeader:
		return titleStyle.Render(input), nil
	case capabilities.TextKindDebug:
		return p.renderTaggedLine("DEBUG", colorHiMagenta, input), nil
	case capabilities.TextKindInfo:
		return p.renderTaggedLine(" INFO ", colorHiCyan, input), nil
	case capabilities.TextKindSuccess:
		return p.renderTaggedLine("  OK  ", colorHiGreen, input), nil
	case capabilities.TextKindWarning:
		return p.renderTaggedLine(" WARN ", colorHiYellow, input), nil
	case capabilities.TextKindError:
		return p.renderTaggedLine(" ERR  ", colorHiRed, input), nil
	case capabilities.TextKindFatal:
		tagStyle := lipgloss.NewStyle().
			Foreground(colorHiWhite).
			Background(colorHiRed).
			Bold(true)
		bodyStyle := lipgloss.NewStyle().
			Foreground(colorHiRed).
			Bold(true)
		return tagStyle.Render("[FATAL ]") + " " + bodyStyle.Render(input), nil
	case capabilities.TextKindMuted:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Render(input), nil
	default:
		return "", fmt.Errorf("style text: unsupported kind %q", kind)
	}
}

// StyleLayout handles semantic layout requests such as panels, splits, and gutters.
func (p *Provider) StyleLayout(kind string, title string, content ...string) (string, error) {
	switch kind {
	case capabilities.LayoutKindPanel:
		renderedTitle := titleStyle.Render(strings.ToUpper(title))
		renderedContent := panelStyle.Render(strings.Join(content, "\n"))
		if title == "" {
			return renderedContent, nil
		}
		return renderedTitle + "\n" + renderedContent, nil
	case capabilities.LayoutKindSplit:
		if len(content) < 2 {
			return "", fmt.Errorf("style layout split: requires at least 2 content blocks")
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, p.withSplitGap(content)...), nil
	case capabilities.LayoutKindGutter:
		if len(content) == 0 {
			return "", fmt.Errorf("style layout gutter: requires at least 1 content block")
		}
		return p.renderGutterLayout(title, content), nil
	default:
		return "", fmt.Errorf("style layout: unsupported kind %q", kind)
	}
}

// StyleTable is intentionally unsupported for charmcli; use prettystyle for tables.
func (p *Provider) StyleTable(kind string, headers []string, rows [][]string) (string, error) {
	_ = kind
	_ = headers
	_ = rows

	return "", fmt.Errorf("style table: charmcli does not render tables; use prettystyle")
}

func (p *Provider) runSelect(
	title string,
	description string,
	options []capabilities.Choice,
	theme *huh.Theme,
) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("style prompt select: options must not be empty")
	}

	result := options[0].Key
	items := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		items = append(items, huh.NewOption(option.Label, option.Key))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(p.styleTitle(title)).
				Description(p.styleDescription(description)).
				Options(items...).
				Value(&result),
		),
	).WithTheme(theme)

	if err := p.execute(form); err != nil {
		return "", fmt.Errorf("style prompt select: %w", err)
	}

	return result, nil
}

func (p *Provider) runToggle(
	title string,
	description string,
	options []capabilities.Choice,
	theme *huh.Theme,
) ([]string, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("style prompt toggle: options must not be empty")
	}

	result := make([]string, 0)
	items := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		items = append(items, huh.NewOption(option.Label, option.Key))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(p.styleTitle(title)).
				Description(p.styleDescription(description)).
				Options(items...).
				Value(&result),
		),
	).WithTheme(theme)

	if err := p.execute(form); err != nil {
		return nil, fmt.Errorf("style prompt toggle: %w", err)
	}

	return result, nil
}

func (p *Provider) runConfirm(title string, description string, theme *huh.Theme) (bool, error) {
	result := false
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(p.styleTitle(title)).
				Description(p.styleDescription(description)).
				Value(&result),
		),
	).WithTheme(theme)

	if err := p.execute(form); err != nil {
		return false, fmt.Errorf("style prompt confirm: %w", err)
	}

	return result, nil
}

func (p *Provider) runInput(title string, description string, theme *huh.Theme) (string, error) {
	result := ""
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(p.styleTitle(title)).
				Description(p.styleDescription(description)).
				Validate(func(value string) error {
					if strings.TrimSpace(value) == "" {
						return fmt.Errorf("input cannot be empty")
					}

					return nil
				}).
				Value(&result),
		),
	).WithTheme(theme)

	if err := p.execute(form); err != nil {
		return "", fmt.Errorf("style prompt input: %w", err)
	}

	return result, nil
}

func (p *Provider) execute(form *huh.Form) error {
	if p.RunForm != nil {
		return p.RunForm(form)
	}

	return form.Run()
}

func (p *Provider) newTheme() *huh.Theme {
	theme := huh.ThemeCharm()
	theme.Focused.Title = lipgloss.NewStyle().Foreground(colorHiMagenta).Bold(true)
	theme.Focused.SelectedOption = lipgloss.NewStyle().Foreground(colorHiMagenta).Bold(true)
	theme.Focused.Description = descStyle
	theme.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(colorHiRed).Bold(true)
	return theme
}

func (p *Provider) styleTitle(title string) string {
	return titleStyle.Render(title)
}

func (p *Provider) styleDescription(description string) string {
	if description == "" {
		return description
	}

	return descStyle.Render(description)
}

func (p *Provider) renderTaggedLine(tag string, color lipgloss.Color, input string) string {
	tagStyle := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
	bodyStyle := lipgloss.NewStyle().
		Foreground(colorHiWhite)
	return tagStyle.Render("["+tag+"]") + " " + bodyStyle.Render(input)
}

func (p *Provider) renderGutterLayout(title string, content []string) string {
	label := strings.ToUpper(strings.TrimSpace(title))
	if label == "" {
		label = "INFO"
	}
	if len(label) > 9 {
		label = label[:9]
	}

	gutter := fmt.Sprintf("%-9s", label)
	lines := make([]string, 0)
	for _, block := range content {
		lines = append(lines, strings.Split(block, "\n")...)
	}

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

func (p *Provider) withSplitGap(content []string) []string {
	if len(content) == 0 {
		return nil
	}

	parts := make([]string, 0, len(content)*2-1)
	for index, block := range content {
		if index > 0 {
			parts = append(parts, splitGap)
		}
		parts = append(parts, block)
	}

	return parts
}
