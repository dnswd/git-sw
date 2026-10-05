package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

func main() {
	// If run with arguments, pass through to git switch
	if len(os.Args) > 1 {
		passThroughToGitSwitch(os.Args[1:])
		return
	}

	// Check if we are in a git repository
	if !isGitRepo() {
		passThroughToGitSwitch(nil)
		return
	}

	// Fetch data for TUI
	runTUI()
}

func passThroughToGitSwitch(args []string) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		fmt.Fprintf(os.Stderr, "git not found in PATH\n")
		os.Exit(1)
	}

	cmdArgs := append([]string{"git", "switch"}, args...)

	// Use syscall.Exec to replace the current process with git switch
	err = syscall.Exec(gitPath, cmdArgs, os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to execute git switch: %v\n", err)
		os.Exit(1)
	}
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	err := cmd.Run()
	return err == nil
}

func runTUI() {
	branches := getBranches()
	if len(branches) <= 1 {
		fmt.Println("You're already in the only local branches, nothing to switch to")
		return
	}

	p := tea.NewProgram(initialModel(branches))
	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	if finalModel, ok := m.(model); ok && finalModel.selectedName != "" {
		passThroughToGitSwitch([]string{finalModel.selectedName})
	}

}

type SortMode int

const (
	SortCheckout SortMode = iota
	SortUpdated
)

type Branch struct {
	Name          string
	Hash          string
	Author        string
	Subject       string
	TimeAgo       string
	CommitterDate int64
	CheckoutDate  int64
}

type model struct {
	branches     []Branch
	filtered     []Branch
	textInput    textinput.Model
	cursor       int
	width        int
	height       int
	sortMode     SortMode
	selectedName string
	quitting     bool
}

func initialModel(branches []Branch) model {
	ti := textinput.New()
	ti.Placeholder = "Search branches..."
	ti.Focus()

	m := model{
		branches:  branches,
		textInput: ti,
		sortMode:  SortCheckout,
		quitting:  false,
	}
	m.sortBranches()
	return m
}

func (m *model) sortBranches() {
	sorted := make([]Branch, len(m.branches))
	copy(sorted, m.branches)

	sort.Slice(sorted, func(i, j int) bool {
		if m.sortMode == SortCheckout {
			if sorted[i].CheckoutDate != sorted[j].CheckoutDate {
				return sorted[i].CheckoutDate > sorted[j].CheckoutDate
			}
			return sorted[i].CommitterDate > sorted[j].CommitterDate
		}
		return sorted[i].CommitterDate > sorted[j].CommitterDate
	})

	m.branches = sorted
	m.filtered = sorted
}

func getBranches() []Branch {
	// Get max 1000 branches
	cmd := exec.Command("git", "for-each-ref", "--sort=-committerdate", "--count=1000",
		"--format=%(refname:short)|%(objectname:short)|%(authorname)|%(subject)|%(committerdate:relative)|%(committerdate:unix)",
		"refs/heads/")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var branches []Branch
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 6 {
			continue
		}
		unixTime, _ := strconv.ParseInt(parts[5], 10, 64)
		branches = append(branches, Branch{
			Name:          parts[0],
			Hash:          parts[1],
			Author:        parts[2],
			Subject:       parts[3],
			TimeAgo:       parts[4],
			CommitterDate: unixTime,
		})
	}

	// Enhance with reflog checkout order
	checkoutOrder := getCheckoutOrder()
	orderMap := make(map[string]int)
	for i, name := range checkoutOrder {
		orderMap[name] = len(checkoutOrder) - i // higher number is more recent
	}

	for i := range branches {
		if order, exists := orderMap[branches[i].Name]; exists {
			branches[i].CheckoutDate = int64(order)
		} else {
			branches[i].CheckoutDate = -1
		}
	}

	return branches
}

func getCheckoutOrder() []string {
	cmd := exec.Command("git", "reflog", "show", "--date=relative", "-n", "1000")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var order []string
	seen := make(map[string]bool)

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "checkout: moving from ") {
			parts := strings.Split(line, " to ")
			if len(parts) == 2 {
				dest := strings.TrimSpace(parts[1])
				// git reflog might append things like " to branch-name\n" so we might need to be careful
				// usually the branch name is until the end of the line
				dest = strings.Split(dest, " ")[0] // simple sanity
				if !seen[dest] {
					seen[dest] = true
					order = append(order, dest)
				}
			}
		}
	}
	return order
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			if m.sortMode == SortCheckout {
				m.sortMode = SortUpdated
			} else {
				m.sortMode = SortCheckout
			}
			m.sortBranches()
			m.filterBranches()
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.filtered) > 0 {
				m.selectedName = m.filtered[m.cursor].Name
			}
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	if len(m.filtered) > 0 {
		m.textInput.Placeholder = m.filtered[m.cursor].Name
	} else {
		m.textInput.Placeholder = "Search branches..."
	}

	m.textInput, cmd = m.textInput.Update(msg)
	// Only refilter if text changed
	if _, ok := msg.(tea.KeyMsg); ok {
		m.filterBranches()
	}
	return m, cmd
}

func (m *model) filterBranches() {
	val := m.textInput.Value()
	if val == "" {
		m.filtered = m.branches
		if m.cursor >= len(m.filtered) {
			m.cursor = len(m.filtered) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return
	}

	var names []string
	for _, b := range m.branches {
		names = append(names, b.Name)
	}

	matches := fuzzy.Find(val, names)
	var filtered []Branch
	for _, match := range matches {
		// match.Index maps to m.branches
		filtered = append(filtered, m.branches[match.Index])
	}

	sort.Slice(filtered, func(i, j int) bool {
		if m.sortMode == SortCheckout {
			if filtered[i].CheckoutDate != filtered[j].CheckoutDate {
				return filtered[i].CheckoutDate > filtered[j].CheckoutDate
			}
			return filtered[i].CommitterDate > filtered[j].CommitterDate
		}
		return filtered[i].CommitterDate > filtered[j].CommitterDate
	})
	m.filtered = filtered

	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

var (
	cursorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	branchStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	timeStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	hashStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	authorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("73"))
	subjectStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	titleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
)

func truncate(s string, max int) string {
	if len(s) > max && max > 3 {
		return s[:max-3] + "..."
	}
	return s
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	s := strings.Builder{}
	s.WriteString(m.textInput.View())
	s.WriteString("\n\n")

	// Calculate display bounds
	listHeight := m.height - 7 // rough overhead
	maxListHeight := m.height * 40 / 100
	if maxListHeight < 10 {
		maxListHeight = 10
	}
	// No hard cap, allow full 40%
	if listHeight > maxListHeight {
		listHeight = maxListHeight
	}

	if listHeight < 5 {
		listHeight = 10 // fallback if height is very small or 0
	}

	// Ensure cursor is visible
	start := 0
	end := len(m.filtered)
	if end > listHeight {
		start = m.cursor - listHeight/2
		if start < 0 {
			start = 0
		}
		end = start + listHeight
		if end > len(m.filtered) {
			end = len(m.filtered)
			start = end - listHeight
		}
	}

	// Dynamic layout dimensions
	maxNameWidth := 0
	hashWidth := 7
	for i := start; i < end; i++ {
		if len(m.filtered[i].Name) > maxNameWidth {
			maxNameWidth = len(m.filtered[i].Name)
		}
		if len(m.filtered[i].Hash) > hashWidth {
			hashWidth = len(m.filtered[i].Hash)
		}
	}

	availWidth := m.width
	if availWidth == 0 {
		availWidth = 80 // fallback
	}

	nameWidth := maxNameWidth
	if nameWidth > availWidth/2 { // Most generous column, but cap at 50%
		nameWidth = availWidth / 2
	}
	if nameWidth < 20 {
		nameWidth = 20
	}

	timeWidth := 15
	authorWidth := 15

	subjectWidth := availWidth - nameWidth - timeWidth - hashWidth - authorWidth - 6
	if subjectWidth < 10 {
		subjectWidth = 10
	}

	for i := start; i < end; i++ {
		b := m.filtered[i]

		cursor := "  "
		bStyle := branchStyle
		if m.cursor == i {
			cursor = "> "
			bStyle = highlightStyle
		}

		name := fmt.Sprintf("%-*s", nameWidth, truncate(b.Name, nameWidth))
		timeAgo := fmt.Sprintf("%-*s", timeWidth, truncate(b.TimeAgo, timeWidth))
		hash := fmt.Sprintf("%-*s", hashWidth, truncate(b.Hash, hashWidth))
		author := fmt.Sprintf("%-*s", authorWidth, truncate(b.Author, authorWidth))
		subject := truncate(b.Subject, subjectWidth)

		row := fmt.Sprintf("%s%s %s %s %s %s\n",
			cursorStyle.Render(cursor),
			bStyle.Render(name),
			timeStyle.Render(timeAgo),
			hashStyle.Render(hash),
			authorStyle.Render(author),
			subjectStyle.Render(subject),
		)
		s.WriteString(row)
	}

	if m.sortMode == SortCheckout {
		s.WriteString("\n[Tab] Sorted by recent checkouts")
	} else {
		s.WriteString("\n[Tab] Sorted by recent updates")
	}
	s.WriteString(" | [Enter] Checkout | [Esc] Quit")
	return s.String()
}
