package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"

	"github.com/kelda/dksnap/pkg/snapshot"
)

type createUI struct {
	client *client.Client

	app               *tview.Application
	containerSelector *ContainerSelector
	*tview.Pages
}

func newCreateUI(client *client.Client, app *tview.Application) *createUI {
	ui := &createUI{
		app:    app,
		client: client,
		Pages:  tview.NewPages(),
	}
	ui.containerSelector = NewContainerSelector(client, ui.promptCreateSnapshot, nil)
	ui.Pages.AddPage("main", ui.containerSelector, true, true)
	return ui
}

func (ui *createUI) run(ctx context.Context) {
	if err := ui.containerSelector.Sync(ctx); err != nil {
		alert(ui.app, ui.Pages, fmt.Sprintf("Failed to list containers: %s", err), nil)
	}
	ui.app.Draw()

	for range newEventsTrigger(ctx, ui.client, "container", "start", "die") {
		if err := ui.containerSelector.Sync(ctx); err != nil {
			continue
		}
		ui.app.Draw()
	}
}

func newEventsTrigger(ctx context.Context, client *client.Client, eventType string, actions ...string) chan struct{} {
	trigger := make(chan struct{}, 1)

	actionsSet := map[string]struct{}{}
	for _, action := range actions {
		actionsSet[action] = struct{}{}
	}

	go func() {
		events, _ := client.Events(ctx, types.EventsOptions{
			Filters: filters.NewArgs(filters.Arg("Type", eventType)),
		})
		for e := range events {
			if _, ok := actionsSet[e.Action]; !ok {
				continue
			}

			select {
			case trigger <- struct{}{}:
			default:
			}
		}
	}()

	return trigger
}

func (ui *createUI) promptCreateSnapshot(container Container) {
	form := tview.NewForm().
		Clear(true)
	form.SetBorder(true).
		SetTitle("Create Snapshot")

	form.
		AddInputField("Title", "", 20, nil, nil).
		AddInputField("Image Name", "", 20, nil, nil).
		AddButton("Create Snapshot", func() {
			title := form.GetFormItemByLabel("Title").(*tview.InputField).GetText()
			imageName := form.GetFormItemByLabel("Image Name").(*tview.InputField).GetText()
			if title == "" || imageName == "" {
				alert(ui.app, ui.Pages, "A title and image name are required.", form)
				return
			}

			var dbUser string
			dbUserInput := form.GetFormItemByLabel("Database User")
			if dbUserInput != nil {
				dbUser = dbUserInput.(*tview.InputField).GetText()
			}

			snapshotLogs := tview.NewTextView().
				SetDynamicColors(true).
				SetChangedFunc(func() {
					ui.app.Draw()
				})

			modalContents := tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(snapshotLogs, 0, 1, false)
			modalContents.
				SetBorder(true).
				SetTitle("Snapshot Status")

			modal := newModal(modalContents, 60, 10)
			ui.Pages.AddPage("snapshot-status", modal, true, true)

			go func() {
				ui.createSnapshot(snapshotLogs, container, title, imageName, dbUser)
				exitButton := tview.NewButton("OK").SetSelectedFunc(func() {
					ui.Pages.RemovePage("snapshot-status")
					ui.Pages.RemovePage("create-snapshot-form")
					ui.app.SetFocus(ui.containerSelector)
				})
				modalContents.AddItem(center(exitButton, 4, 1), 0, 1, true)
				ui.app.SetFocus(exitButton)
			}()
		})

	titleInput := form.GetFormItemByLabel("Title").(*tview.InputField)
	imageNameInput := form.GetFormItemByLabel("Image Name").(*tview.InputField)
	submitButton := form.GetButton(form.GetButtonIndex("Create Snapshot"))
	inputFields := []*tview.InputField{
		titleInput,
		imageNameInput,
	}

	// We need the user to dump as when taking Postgres snapshots.
	if container.HasPostgres {
		dbUser, ok := getEnv(container.Config.Env, "POSTGRES_USER")
		if !ok {
			dbUser = "postgres"
		}

		form.AddInputField("Database User", dbUser, 20, nil, nil)
		inputFields = append(inputFields, form.GetFormItemByLabel("Database User").(*tview.InputField))
	}

	// Automatically generate image names based on the snapshot title.
	titleInput.SetChangedFunc(func(name string) {
		image := strings.ToLower(name)

		// Convert spaces into a legal separator.
		image = strings.Replace(image, " ", "-", -1)

		// Remove all other illegal characters.
		image = regexp.MustCompile(`[^\w.-]`).ReplaceAllString(image, "")

		imageNameInput.SetText(image)
	})

	// Allow navigating between the fields with arrow keys.
	for i, inputField := range inputFields {
		i := i
		isFirstField := i == 0
		isLastField := i == len(inputFields)-1
		inputField.SetInputCapture(
			func(event *tcell.EventKey) *tcell.EventKey {
				nextInput := func() {
					if isLastField {
						ui.app.SetFocus(submitButton)
						return
					}

					target := (i + 1) % len(inputFields)
					ui.app.SetFocus(inputFields[target])
				}

				prevInput := func() {
					if isFirstField {
						ui.app.SetFocus(submitButton)
						return
					}

					target := (i - 1) % len(inputFields)
					ui.app.SetFocus(inputFields[target])
				}

				switch event.Key() {
				case tcell.KeyUp:
					prevInput()
					return nil
				case tcell.KeyDown:
					nextInput()
					return nil
				default:
					return event
				}
			})
	}

	submitButton.SetInputCapture(
		func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyUp:
				ui.app.SetFocus(inputFields[len(inputFields)-1])
				return nil
			case tcell.KeyDown:
				ui.app.SetFocus(inputFields[0])
				return nil
			default:
				return event
			}
		})

	// Show the form.
	ui.Pages.AddPage("create-snapshot-form", newModal(form, 50, 20), true, true)
	ui.app.SetFocus(form)
	form.SetCancelFunc(func() {
		ui.Pages.RemovePage("create-snapshot-form")
	})
}

// createSnapshot takes a snapshot of the given container. It attempts to use
// the database aware snapshot implementation first, but falls back to a
// generic snapshot if that fails.
func (ui *createUI) createSnapshot(out *tview.TextView, container Container, title, imageName, dbUser string) {
	fmt.Fprintf(out, "Creating snapshot..")
	pp := NewProgressPrinter(out)
	pp.Start()

	// Take the database aware snapshot.
	var snapshotter snapshot.Snapshotter
	switch {
	case container.HasPostgres:
		snapshotter = snapshot.NewPostgres(ui.client, dbUser)
	case container.HasMongo:
		snapshotter = snapshot.NewMongo(ui.client)
	case container.HasMySQL:
		snapshotter = snapshot.NewMySQL(ui.client)
	default:
		snapshotter = snapshot.NewGeneric(ui.client)
	}

	err := snapshotter.Create(context.Background(), container.ContainerJSON, title, imageName)
	pp.Stop()
	out.Clear()
	out.SetTextAlign(tview.AlignCenter)

	if err == nil {
		fmt.Fprintln(out, "[green]Successfully created snapshot![-]")
		return
	}
	fmt.Fprintf(out, "[red]Failed to create snapshot[-]\n%s", err)

	// Don't try snapshotting again if we already tried the generic snapshot.
	if _, ok := snapshotter.(*snapshot.Generic); ok {
		return
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "[yellow]Falling back to using a generic snapshot..")
	pp.Start()
	err = snapshot.NewGeneric(ui.client).Create(context.Background(), container.ContainerJSON, title, imageName)
	pp.Stop()
	if err == nil {
		fmt.Fprintln(out, "[green]Successfully created snapshot![-]")
	} else {
		fmt.Fprintf(out, "[red]Failed to create snapshot[-]\n%s", err)
	}
}

// Returns a new primitive which puts the provided primitive in the center and
// sets its size to the given width and height.
func newModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, false).
			AddItem(nil, 0, 1, false), width, 1, false).
		AddItem(nil, 0, 1, false)
}

// Returns a new primitive which centers the provided primitive and
// sets its size to the given width and height.
func center(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewBox(), 0, 1, false).
			AddItem(p, height, 1, false).
			AddItem(tview.NewBox(), 0, 1, false), width, 1, false).
		AddItem(tview.NewBox(), 0, 1, false)
}

func getEnv(vars []string, key string) (string, bool) {
	for _, env := range vars {
		envParts := strings.Split(env, "=")
		if len(envParts) != 2 {
			continue
		}
		if envParts[0] == key {
			return envParts[1], true
		}
	}
	return "", false
}
