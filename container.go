package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/go-units"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

type Container struct {
	HasPostgres bool
	HasMongo    bool
	types.ContainerJSON
}

type ContainerSelector struct {
	selectedFunc func(Container)
	client       *client.Client
	*tview.Table
}

const (
	containerImageColumnIndex = iota
	containerCreatedColumnIndex
	containerNameColumnIndex
)

func NewContainerSelector(client *client.Client, selectedFunc func(Container), doneFunc func(tcell.Key)) *ContainerSelector {
	table := tview.NewTable().
		SetDoneFunc(doneFunc)
	return &ContainerSelector{
		client:       client,
		selectedFunc: selectedFunc,
		Table:        table,
	}
}

func (cs *ContainerSelector) Sync(ctx context.Context) error {
	containerIDs, err := cs.client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	var containers []Container
	for _, containerID := range containerIDs {
		containerInfo, err := cs.client.ContainerInspect(ctx, containerID.ID)
		if err != nil {
			return err
		}

		hasPostgres := false
		hasMongo := false

		topResp, err := cs.client.ContainerTop(ctx, containerID.ID, []string{"-eo", "pid,comm"})
		if err != nil {
			// The container was stopped between the list and top.
			if strings.Contains(err.Error(), "is not running") {
				continue
			}
		} else {
			for _, process := range topResp.Processes {
				if len(process) != 2 {
					continue
				}

				if strings.Contains(process[1], "postgres") {
					hasPostgres = true
					break
				} else if strings.Contains(process[1], "mongo") {
					hasMongo = true
					break
				}
			}
		}

		// Reference the image by the user-friendly name.
		containerInfo.Image = containerID.Image
		containers = append(containers, Container{
			HasPostgres:   hasPostgres,
			HasMongo:      hasMongo,
			ContainerJSON: containerInfo,
		})
	}

	cs.draw(containers)
	return nil
}

func (cs *ContainerSelector) draw(containers []Container) {
	cs.Clear().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetBorder(true).
		SetTitle("Containers")

	// Set column names.
	// TODO: Add field for snapshot name.
	cs.SetCell(0, containerImageColumnIndex, &tview.TableCell{
		Text:          "IMAGE",
		Color:         tcell.ColorYellow,
		Expansion:     1,
		NotSelectable: true,
	})
	cs.SetCell(0, containerCreatedColumnIndex, &tview.TableCell{
		Text:          "CREATED",
		Color:         tcell.ColorYellow,
		Expansion:     1,
		NotSelectable: true,
	})
	cs.SetCell(0, containerNameColumnIndex, &tview.TableCell{
		Text:          "NAME",
		Color:         tcell.ColorYellow,
		Expansion:     1,
		NotSelectable: true,
	})

	// Populate each row of the table with the container information.
	for i, container := range containers {
		var created string
		createdTime, err := time.Parse(time.RFC3339Nano, container.Created)
		if err == nil {
			created = units.HumanDuration(time.Since(createdTime)) + " ago"
		} else {
			created = fmt.Sprintf("Unknown: %s", err)
		}

		// Skip the column names in the first row.
		row := i + 1
		cs.SetCellSimple(row, containerImageColumnIndex, container.Image)
		cs.SetCellSimple(row, containerCreatedColumnIndex, created)
		cs.SetCellSimple(row, containerNameColumnIndex, strings.TrimPrefix(container.Name, "/"))
	}

	cs.SetSelectedFunc(func(i, _ int) {
		// The first row is taken by the column names.
		containerIndex := i - 1
		cs.selectedFunc(containers[containerIndex])
	})
}
