package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const (
	initialInputs = 2
	maxInputs     = 6
	minInputs     = 1
	titleHeight   = 3
	helpHeight    = 5
)

var (
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	cursorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("57")).
			Foreground(lipgloss.Color("230"))

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238"))

	endOfBufferStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235"))

	focusedPlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))

	blurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.HiddenBorder())
)

type keymap = struct {
	next, insertComponent, prev, add, remove, save, quit key.Binding
}

func newTextarea() textarea.Model {
	t := textarea.New()
	t.Prompt = ""
	t.Placeholder = "Type something"
	t.ShowLineNumbers = true
	t.Cursor.Style = cursorStyle
	t.FocusedStyle.Placeholder = focusedPlaceholderStyle
	t.BlurredStyle.Placeholder = placeholderStyle
	t.FocusedStyle.CursorLine = cursorLineStyle
	t.FocusedStyle.Base = focusedBorderStyle
	t.BlurredStyle.Base = blurredBorderStyle
	t.FocusedStyle.EndOfBuffer = endOfBufferStyle
	t.BlurredStyle.EndOfBuffer = endOfBufferStyle
	t.KeyMap.DeleteWordBackward.SetEnabled(false)
	t.KeyMap.LineNext = key.NewBinding(key.WithKeys("down"))
	t.KeyMap.LinePrevious = key.NewBinding(key.WithKeys("up"))
	t.Blur()
	return t
}

type model struct {
	width     int
	height    int
	keymap    keymap
	help      help.Model
	input     textarea.Model
	viewport  viewport.Model
	focus     int
	stopwatch stopwatch.Model
	title     string
	filePath  string
}

func newModel(filePath string) model {
	m := model{
		input:     newTextarea(),
		viewport:  viewport.New(0, 0),
		help:      help.New(),
		title:     "A New File",
		stopwatch: stopwatch.NewWithInterval(time.Second),
		filePath:  filePath,
		keymap: keymap{
			quit: key.NewBinding(
				key.WithKeys("esc", "ctrl+c", "cmd+q"),
				key.WithHelp("esc", "quit"),
			),
			save: key.NewBinding(
				key.WithKeys("ctrl+s", "cmd+s"),
				key.WithHelp("ctrl+s", "save a file"),
			),
			insertComponent: key.NewBinding(
				key.WithKeys("ctrl+i", "cmd+i"),
				key.WithHelp("ctrl+i", "insert md component"),
			),
		},
	}

	m.updateKeybindings()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.stopwatch.Init(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.quit):
			m.input.Blur()
			return m, tea.Quit
		case key.Matches(msg, m.keymap.save):
			saveFile(m)
		default:
			if !m.input.Focused() {
				cmd := m.input.Focus()
				cmds = append(cmds, cmd)

			}
		}

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.sizeInputs()
	}

	m.updateKeybindings()

	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		swCmd tea.Cmd
	)
	m.input, tiCmd = m.input.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	m.stopwatch, swCmd = m.stopwatch.Update(msg)

	cmds = append(cmds, tiCmd, vpCmd, swCmd)
	return m, tea.Batch(cmds...)
}

func (m *model) sizeInputs() {
	m.input.SetWidth(m.width / 2)
	m.input.SetHeight(m.height - helpHeight - titleHeight)

	m.viewport = viewport.New(m.width/2, m.height-helpHeight-titleHeight)
}

func (m *model) updateKeybindings() {
	// m.keymap.add.SetEnabled(len(m.inputs) < maxInputs)
	// m.keymap.remove.SetEnabled(len(m.inputs) > minInputs)
}

var (
	L = lipgloss.Left
	R = lipgloss.Right

	titleStyle     = lipgloss.NewStyle().Bold(true).Align(L).Padding(1, 1)
	bufferStyle    = lipgloss.NewStyle()
	stopwatchStyle = lipgloss.NewStyle().Bold(true).Align(R).Padding(1, 1)
)

func (m model) View() string {
	page := strings.Builder{}

	title := titleStyle.Render(m.title)
	sw := stopwatchStyle.Render(m.stopwatch.View())
	buffer := bufferStyle.Width(m.width - lipgloss.Width(title) - lipgloss.Width(sw)).Render(" ")

	titleBar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		title,
		buffer,
		sw,
	)
	page.WriteString(titleBar)

	help := m.help.ShortHelpView([]key.Binding{
		m.keymap.next,
		m.keymap.prev,
		m.keymap.add,
		m.keymap.remove,
		m.keymap.quit,
	})

	renderedMarkdown, _ := glamour.Render(m.input.Value(), "dark")

	m.viewport.SetContent(renderedMarkdown)

	// Need to style left and right sides
	// 1. have a gutter between
	// 2. Nice padding
	// 3. Highlight current line

	page.WriteString("\n\n")
	page.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, m.input.View(), m.viewport.View()))
	page.WriteString("\n\n")
	page.WriteString(help)
	return page.String()
}

func saveFile(m model) {
	b := strings.Builder{}

	// Front matter
	homePath, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Could not find user")
	}

	split := strings.Split(homePath, "/")
	userName := split[len(split)-1]

	frontMatterData := map[string]string{
		"user": userName,
		"time": m.stopwatch.Elapsed().String(),
	}

	frontMatterTemplate, err := template.New("").Parse(`---
{{ range $k, $v := . }}{{$k}} = "{{$v}}"{{ end }}
---
`)
	if err != nil {
		log.Fatalf("Failed to generate front matter | %v", err)
	}

	var bytesBuffer bytes.Buffer
	if err := frontMatterTemplate.Execute(&bytesBuffer, frontMatterData); err != nil {
		log.Fatalf("Failed to render template")
	}

	b.WriteString(bytesBuffer.String())

	// Markdown content

	b.WriteString(m.input.Value())

	os.WriteFile(m.filePath, []byte(b.String()), 0666)
}

func main() {

	filePath := flag.String("file-path", "", "path to markdown file")
	flag.Parse()

	if *filePath == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := tea.NewProgram(newModel(*filePath), tea.WithAltScreen()).Start(); err != nil {
		fmt.Println("Error while running program:", err)
		os.Exit(1)
	}
}