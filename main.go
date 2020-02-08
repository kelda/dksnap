package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/docker/docker/client"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

var forceGenericSnapshot = flag.Bool("force-generic", false, "disable database aware snapshots")

func main() {
	flag.Parse()

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Docker client: %s\n", err)
		os.Exit(1)
	}

	app := tview.NewApplication()
	createUI := newCreateUI(dockerClient, app)
	infoUI := newInfoUI(dockerClient, app)

	ctx := context.Background()
	go createUI.run(ctx)
	go infoUI.run(ctx)

	// Setup tab navigation. The tab's index in the following list is used as
	// the tab's identifier in the Pages view, and as the Region in the tab
	// tracker.
	tabs := []struct {
		title    string
		contents tview.Primitive
	}{
		{"Create Snapshot", createUI},
		{"View Snapshots", infoUI},
	}
	tabTracker := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)

	currentTab := 0
	tabTracker.Highlight(strconv.Itoa(currentTab))
	tabbedView := tview.NewPages()
	previousTab := func() {
		currentTab = (currentTab - 1 + len(tabs)) % len(tabs)
		tabTracker.Highlight(strconv.Itoa(currentTab)).
			ScrollToHighlight()
		tabbedView.SwitchToPage(strconv.Itoa(currentTab))
	}
	nextTab := func() {
		currentTab = (currentTab + 1) % len(tabs)
		tabTracker.Highlight(strconv.Itoa(currentTab)).
			ScrollToHighlight()
		tabbedView.SwitchToPage(strconv.Itoa(currentTab))
	}

	fmt.Fprintf(tabTracker, "[yellow]TABS[-] |")
	for index, tab := range tabs {
		tabbedView.AddPage(strconv.Itoa(index), tab.contents, true, index == currentTab)
		fmt.Fprintf(tabTracker, ` ["%d"][darkcyan]%s[-][""] |`, index, tab.title)
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlP:
			previousTab()
			return nil
		case tcell.KeyCtrlN:
			nextTab()
			return nil
		default:
			return event
		}
	})

	controls := NewControlsView([]KeyMapping{
		{"Ctrl-C", "Quit"},
		{"Ctrl-N", "Next tab"},
		{"Ctrl-P", "Previous tab"},
		{"↑↓←→", "Select item"},
		{"ENTER", "Pick item"},
		{"ESC", "Cancel"},
	})

	// Show the tab tracker at the top of the screen, followed by the tab
	// contents, and the controls at the bottom.
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tabTracker, 1, 1, false).
		AddItem(tabbedView, 0, 1, true).
		AddItem(controls, 2, 1, false)
	if err := app.SetRoot(root, true).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to view snapshots: %s\n", err)
	}
}

// KeyMapping represents a control used to interact with the UI.
type KeyMapping struct {
	Key    string
	Action string
}

// NewControlsView creates a new tview component that displays the given controls.
func NewControlsView(controls []KeyMapping) *tview.TextView {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]NAVIGATION[-] |")
	for _, mapping := range controls {
		fmt.Fprintf(text, " [darkcyan]%s[-] %s |", mapping.Key, mapping.Action)
	}
	return text
}

func alert(app *tview.Application, root *tview.Pages, message string, focusAfter tview.Primitive) {
	root.AddPage("alert-modal", tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetButtonBackgroundColor(buttonColor).
		SetDoneFunc(func(_ int, _ string) {
			root.RemovePage("alert-modal")
			if focusAfter != nil {
				app.SetFocus(focusAfter)
			}
		}), true, true)
}
