package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/rivo/tview"

	"github.com/kelda/docker-snapshot/pkg/snapshot"
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

	// TODO: Allow moving between forms with up and down arrow.
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

			fmt.Fprintf(snapshotLogs, "Creating snapshot..")
			pp := NewProgressPrinter(snapshotLogs)
			pp.Start()

			go func() {
				var err error
				switch {
				case container.HasPostgres:
					dbUser := form.GetFormItemByLabel("Database User").(*tview.InputField).GetText()
					err = snapshot.SnapshotPostgres(context.Background(), ui.client, container.ContainerJSON, title, imageName, dbUser)
				case container.HasMongo:
					err = snapshot.SnapshotMongo(context.Background(), ui.client, container.ContainerJSON, title, imageName)
				default:
					err = snapshot.SnapshotGeneric(context.Background(), ui.client, container.ContainerJSON, title, imageName)
				}

				pp.Stop()
				snapshotLogs.Clear()
				snapshotLogs.SetTextAlign(tview.AlignCenter)

				message := "[green]Successfully created snapshot![-]"
				if err != nil {
					message = fmt.Sprintf("[red]Failed to create snapshot[-]\n%s", err)
				}
				fmt.Fprintln(snapshotLogs, message)

				exitButton := tview.NewButton("OK").SetSelectedFunc(func() {
					ui.Pages.RemovePage("snapshot-status")
					if err == nil {
						ui.Pages.RemovePage("create-snapshot-form")
						ui.app.SetFocus(ui.containerSelector)
					} else {
						ui.app.SetFocus(form)
					}
				})
				modalContents.AddItem(center(exitButton, 4, 1), 0, 1, true)
				ui.app.SetFocus(exitButton)
			}()
		})

	// TODO: Test mongo auth.
	if container.HasPostgres {
		dbUser, ok := getEnv(container.Config.Env, "POSTGRES_USER")
		if !ok {
			dbUser = "postgres"
		}

		form.AddInputField("Database User", dbUser, 20, nil, nil)
	}

	titleInput := form.GetFormItemByLabel("Title").(*tview.InputField)
	imageNameInput := form.GetFormItemByLabel("Image Name").(*tview.InputField)
	titleInput.SetChangedFunc(func(name string) {
		image := strings.ToLower(name)

		// Convert spaces into a legal separator.
		image = strings.Replace(image, " ", "-", -1)

		// Remove all other illegal characters.
		image = regexp.MustCompile(`[^\w.-]`).ReplaceAllString(image, "")

		imageNameInput.SetText(image)
	})

	formModal := newModal(form, 50, 20)
	ui.Pages.AddPage("create-snapshot-form", formModal, true, true)
	ui.app.SetFocus(form)
	form.SetCancelFunc(func() {
		ui.Pages.RemovePage("create-snapshot-form")
	})
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
