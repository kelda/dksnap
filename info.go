package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-units"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"

	"github.com/kelda/dksnap/pkg/snapshot"
)

const buttonColor = tcell.ColorDarkCyan

type infoUI struct {
	client           *client.Client
	snapshots        []*snapshot.Snapshot
	selectedSnapshot *snapshot.Snapshot

	app                 *tview.Application
	snapshotActionsView *tview.Flex
	snapshotListView    *tview.Table
	*tview.Pages
}

func newInfoUI(dockerClient *client.Client, app *tview.Application) *infoUI {
	ui := &infoUI{
		client:              dockerClient,
		snapshotListView:    tview.NewTable(),
		snapshotActionsView: tview.NewFlex(),
		Pages:               tview.NewPages(),
		app:                 app,
	}
	ui.setupSnapshotActions()

	ui.snapshotListView.SetSelectedFunc(func(row, _ int) {
		snapshotIndex := row - 1
		if snapshotIndex < 0 || snapshotIndex >= len(ui.snapshots) {
			return
		}
		ui.selectedSnapshot = ui.snapshots[snapshotIndex]
		ui.app.SetFocus(ui.snapshotActionsView)
	})

	ui.Pages.AddPage("main", tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.snapshotListView, 0, 1, true).
		AddItem(ui.snapshotActionsView, 1, 1, true),
		true, true)
	return ui
}

func (ui *infoUI) run(ctx context.Context) {
	if err := ui.syncSnapshots(ctx); err != nil {
		alert(ui.app, ui.Pages, fmt.Sprintf("Failed to list snapshots: %s", err), nil)
	}

	for range newEventsTrigger(ctx, ui.client, "image", "tag", "delete", "untag") {
		if err := ui.syncSnapshots(ctx); err != nil {
			continue
		}
		ui.app.Draw()
	}
}

const (
	snapshotNameColumnIndex = iota
	snapshotImageColumnIndex
	snapshotCreatedColumnIndex
)

func (ui *infoUI) renderSnapshotList() {
	ui.snapshotListView.
		SetSelectable(true, false).
		SetFixed(1, 0).
		Clear().
		SetBorder(true).
		SetTitle("Snapshots")

	// Set column names.
	ui.snapshotListView.SetCell(0, snapshotImageColumnIndex, &tview.TableCell{
		Text:          "IMAGE",
		Color:         tcell.ColorYellow,
		Expansion:     1,
		NotSelectable: true,
	})
	ui.snapshotListView.SetCell(0, snapshotNameColumnIndex, &tview.TableCell{
		Text:          "NAME",
		Color:         tcell.ColorYellow,
		Expansion:     1,
		NotSelectable: true,
	})
	ui.snapshotListView.SetCell(0, snapshotCreatedColumnIndex, &tview.TableCell{
		Text:          "CREATED",
		Color:         tcell.ColorYellow,
		Expansion:     1,
		NotSelectable: true,
	})

	// Populate each row of the table with the container information.
	for idx, snapshot := range ui.snapshots {
		// Skip the column names in the first row.
		row := idx + 1
		ui.snapshotListView.SetCellSimple(row, snapshotImageColumnIndex, strings.Join(snapshot.ImageNames, ", "))
		ui.snapshotListView.SetCellSimple(row, snapshotNameColumnIndex, snapshot.Title)
		ui.snapshotListView.SetCellSimple(row, snapshotCreatedColumnIndex, units.HumanDuration(
			time.Since(snapshot.Created))+" ago")
	}
	ui.app.Draw()
}

func (ui *infoUI) popupHistory(snap *snapshot.Snapshot) {
	diffView := tview.NewTextView().
		SetScrollable(true).
		SetDynamicColors(true).
		SetChangedFunc(func() {
			ui.app.Draw()
		})
	diffView.SetBorder(true).SetTitle("Diff")

	treeView := tview.NewTreeView()
	treeView.SetBorder(true).SetTitle("History")

	root := snap
	for root.Parent != nil {
		root = root.Parent
	}

	rootNode := tview.NewTreeNode(snapshotNodeName(root)).SetReference(root)
	treeView.SetRoot(rootNode)
	addChildren(rootNode, root.Children)

	var selectedNode *tview.TreeNode
	rootNode.Walk(func(node, _ *tview.TreeNode) bool {
		if node.GetReference() == snap {
			selectedNode = node
			return false
		}
		return true
	})
	if selectedNode != nil {
		selectedNode.SetColor(tcell.ColorGreen)
		treeView.SetCurrentNode(selectedNode)
	}

	treeView.SetDoneFunc(func(_ tcell.Key) {
		ui.Pages.RemovePage("snapshot-history")
		ui.app.SetFocus(ui.snapshotActionsView)
	})

	treeView.SetSelectedFunc(func(selected *tview.TreeNode) {
		ui.app.SetFocus(diffView)
	})

	diffView.SetDoneFunc(func(_ tcell.Key) {
		ui.app.SetFocus(treeView)
	})

	treeView.SetChangedFunc(func(selected *tview.TreeNode) {
		ui.renderDiff(diffView, selected.GetReference().(*snapshot.Snapshot), ui.selectedSnapshot)
	})

	historyView := tview.NewFlex().
		AddItem(treeView, 0, 1, true).
		AddItem(diffView, 0, 1, false)
	ui.Pages.AddAndSwitchToPage("snapshot-history", historyView, true)
	ui.app.SetFocus(historyView)
}

func (ui *infoUI) popupReplaceContainer(snap *snapshot.Snapshot) {
	selectedFunc := func(container Container) {
		logs := tview.NewTextView().
			SetDynamicColors(true).
			SetChangedFunc(func() {
				ui.app.Draw()
			})

		modalContents := tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(logs, 0, 3, false)
		modalContents.
			SetBorder(true).
			SetTitle("Boot Status")

		modal := newModal(modalContents, 60, 10)
		ui.Pages.AddPage("replace-status", modal, true, true)

		fmt.Fprintf(logs, "Replacing container..")
		pp := NewProgressPrinter(logs)
		pp.Start()

		go func() {
			err := ui.replaceContainer(context.Background(), container, snap)
			ui.app.QueueUpdateDraw(func() {
				pp.Stop()
				logs.Clear()
				logs.SetTextAlign(tview.AlignCenter)

				message := "[green]Successfully replaced container![-]"
				if err != nil {
					message = fmt.Sprintf("[red]Failed to replace container:[-]\n%s", err)
				}
				fmt.Fprintln(logs, message)

				exitButton := tview.NewButton("OK").SetSelectedFunc(func() {
					ui.Pages.RemovePage("replace-status")
					ui.Pages.RemovePage("replace-container-modal")
				})
				modalContents.AddItem(center(exitButton, 4, 1), 0, 1, true)
				ui.app.SetFocus(exitButton)
			})
		}()
	}
	doneFunc := func(_ tcell.Key) {
		ui.Pages.RemovePage("replace-container-modal")
	}

	containerSelector := NewContainerSelector(ui.client, selectedFunc, doneFunc)
	if err := containerSelector.Sync(context.Background()); err != nil {
		alert(ui.app, ui.Pages, fmt.Sprintf("Failed to list containers: %s", err), ui.snapshotActionsView)
		return
	}

	_, _, screenWidth, screenHeight := ui.Pages.GetRect()
	modal := newModal(containerSelector, screenWidth-10, screenHeight-10)
	ui.Pages.AddAndSwitchToPage("replace-container-modal", modal, true)
	ui.app.SetFocus(containerSelector)
}

func (ui *infoUI) replaceContainer(ctx context.Context, old Container, snap *snapshot.Snapshot) error {
	err := ui.client.ContainerRemove(ctx, old.ID, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return fmt.Errorf("remove old container: %w", err)
	}

	containerConfig := old.Config
	containerConfig.Image = snap.ImageID
	if len(snap.ImageNames) > 0 {
		containerConfig.Image = snap.ImageNames[0]
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: old.NetworkSettings.Networks,
	}
	createdContainer, err := ui.client.ContainerCreate(ctx, containerConfig, old.HostConfig, networkingConfig, old.Name)
	if err != nil {
		return fmt.Errorf("create new container: %w", err)
	}

	err = ui.client.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("start new container: %w", err)
	}
	return nil
}

func (ui *infoUI) renderDiff(diffView *tview.TextView, oldSnap, newSnap *snapshot.Snapshot) {
	if oldSnap == newSnap {
		diffView.SetText("")
		return
	}

	fmt.Fprintf(diffView, "Generating diff..")
	pp := NewProgressPrinter(diffView)
	pp.Start()

	go func() {
		diff, err := snapshot.Diff(context.Background(), ui.client, oldSnap, newSnap)
		pp.Stop()
		if err != nil {
			diffView.SetText(fmt.Sprintf("Failed to diff: %s", err))
			return
		}

		ui.app.QueueUpdateDraw(func() {
			diffView.SetText(colorizeDiff(diff))
		})
	}()
}

func (ui *infoUI) setupSnapshotActions() {
	label := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow]ACTIONS[-]    |")
	ui.snapshotActionsView.Clear().
		AddItem(label, 13, 0, false)

	historyButton := tview.NewButton("View Snapshot History").
		SetSelectedFunc(func() {
			ui.popupHistory(ui.selectedSnapshot)
		})

	bootButton := tview.NewButton("Boot New Container").
		SetSelectedFunc(func() {
			if err := ui.bootSnapshot(context.Background(), ui.selectedSnapshot); err != nil {
				alert(ui.app, ui.Pages, fmt.Sprintf("Failed to boot snapshot: %s", err), ui.snapshotActionsView)
			} else {
				alert(ui.app, ui.Pages, "Successfully booted snapshot", ui.snapshotActionsView)
			}
		})

	replaceButton := tview.NewButton("Replace Running Container").
		SetSelectedFunc(func() {
			ui.popupReplaceContainer(ui.selectedSnapshot)
		})

	deleteButton := tview.NewButton("Delete Snapshot").
		SetSelectedFunc(func() {
			for _, name := range ui.selectedSnapshot.ImageNames {
				_, err := ui.client.ImageRemove(context.Background(), name, types.ImageRemoveOptions{Force: true})
				if err != nil {
					alert(ui.app, ui.Pages, fmt.Sprintf("Failed to delete snapshot: %s", err), ui.snapshotActionsView)
				} else {
					alert(ui.app, ui.Pages, "Successfully deleted snapshot", ui.snapshotActionsView)
				}
			}
		})

	buttons := []*tview.Button{
		historyButton, bootButton, replaceButton, deleteButton,
	}
	for i, button := range buttons {
		i := i
		button.SetInputCapture(
			func(event *tcell.EventKey) *tcell.EventKey {
				changeButton := func(delta int) {
					// Add len(buttons) so that we never go negative.
					target := (i + delta + len(buttons)) % len(buttons)
					ui.app.SetFocus(buttons[target])
				}

				switch event.Key() {
				case tcell.KeyLeft:
					changeButton(-1)
					return nil
				case tcell.KeyRight:
					changeButton(1)
					return nil
				case tcell.KeyEsc:
					ui.app.SetFocus(ui.snapshotListView)
					return nil
				default:
					return event
				}
			})

		button.
			SetBackgroundColor(buttonColor).
			SetBorderPadding(0, 0, 1, 1)
		paddingBox := tview.NewBox()
		paddingBox.SetRect(0, 0, 1, 0)
		width := len(button.GetLabel()) + 2
		ui.snapshotActionsView.
			AddItem(button, width, 0, i == 0).
			AddItem(paddingBox, 1, 0, false)
	}

	ui.app.Draw()
}

func (ui *infoUI) bootSnapshot(ctx context.Context, snap *snapshot.Snapshot) error {
	image := snap.ImageID
	if len(snap.ImageNames) > 0 {
		image = snap.ImageNames[0]
	}

	containerSpec := &container.Config{Image: image}
	containerID, err := ui.client.ContainerCreate(ctx, containerSpec, nil, nil, "")
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	err = ui.client.ContainerStart(ctx, containerID.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	return nil
}

func (ui *infoUI) syncSnapshots(ctx context.Context) error {
	snapshots, err := snapshot.List(ctx, ui.client)
	if err != nil {
		return fmt.Errorf("list snapshots: %w", err)
	}

	ui.snapshots = snapshots
	ui.renderSnapshotList()
	return nil
}

func addChildren(parent *tview.TreeNode, children []*snapshot.Snapshot) {
	for _, child := range children {
		childNode := tview.NewTreeNode(snapshotNodeName(child)).SetReference(child)
		parent.AddChild(childNode)
		addChildren(childNode, child.Children)
	}
}

func colorizeDiff(toColorize string) string {
	var colorized bytes.Buffer
	for _, line := range strings.SplitAfter(toColorize, "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			colorized.WriteString(fmt.Sprintf("[green]%s[::-]", line))
		case strings.HasPrefix(line, "-"):
			colorized.WriteString(fmt.Sprintf("[red]%s[::-]", line))
		default:
			colorized.WriteString(line)
		}
	}
	return colorized.String()
}

func snapshotNodeName(snap *snapshot.Snapshot) string {
	if !snap.BaseImage {
		return snap.Title
	}

	if len(snap.ImageNames) > 0 {
		return snap.ImageNames[0]
	}
	return snap.ImageID
}
