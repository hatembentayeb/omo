package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (dv *DockerView) newImagesView() *ui.Cores {
	cores := ui.NewCores(dv.app, "Docker Images")
	cores.SetTableHeaders([]string{"ID", "Repository", "Tag", "Size", "Created"})
	cores.SetRefreshCallback(dv.refreshImagesData)
	cores.SetSelectionKey("ID")

	// Navigation key bindings
	cores.AddKeyBinding("C", "Containers", dv.showContainers)
	cores.AddKeyBinding("I", "Images", dv.showImages)
	cores.AddKeyBinding("N", "Networks", dv.showNetworks)
	cores.AddKeyBinding("V", "Volumes", dv.showVolumes)
	cores.AddKeyBinding("T", "Stats", dv.showStats)
	cores.AddKeyBinding("O", "Compose", dv.showCompose)
	cores.AddKeyBinding("Y", "System", dv.showSystem)

	// Image action key bindings
	cores.AddKeyBinding("D", "Delete", dv.removeSelectedImage)
	cores.AddKeyBinding("P", "Pull", dv.pullImage)
	cores.AddKeyBinding("H", "History", dv.showImageHistory)
	cores.AddKeyBinding("R", "Run", dv.runImageAsContainer)
	cores.AddKeyBinding("?", "Help", dv.showHelp)

	cores.SetActionCallback(dv.handleAction)

	// Set row selection callback
	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected image: %s:%s", tableData[row][1], tableData[row][2]))
		}
	})

	// Set Enter key to show image details
	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		dv.showImageDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (dv *DockerView) refreshImagesData() ([][]string, error) {
	if dv.dockerClient == nil {
		return [][]string{}, fmt.Errorf("docker client not initialized")
	}

	images, err := dv.dockerClient.ListImages()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(images))
	for i, image := range images {
		rows[i] = image.GetTableRow()
	}

	if dv.currentHost != nil {
		dv.imagesView.SetInfoText(fmt.Sprintf("[green]Docker Images[white]\nHost: %s\nImages: %d\nStatus: Connected",
			dv.currentHost.Name, len(images)))
	}

	return rows, nil
}

func (dv *DockerView) getSelectedImageID() (string, bool) {
	row := dv.imagesView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return row[0], true
}

func (dv *DockerView) getSelectedImageName() string {
	row := dv.imagesView.GetSelectedRowData()
	if len(row) < 3 {
		return ""
	}
	return fmt.Sprintf("%s:%s", row[1], row[2])
}

func (dv *DockerView) removeSelectedImage() {
	id, ok := dv.getSelectedImageID()
	if !ok {
		dv.imagesView.Log("[yellow]No image selected")
		return
	}

	name := dv.getSelectedImageName()

	ui.ShowStandardConfirmationModal(
		dv.pages,
		dv.app,
		"Remove Image",
		fmt.Sprintf("Are you sure you want to remove image [red]%s[white]?", name),
		func(confirmed bool) {
			if confirmed {
				dv.imagesView.Log(fmt.Sprintf("[yellow]Removing image %s...", name))
				if err := dv.dockerClient.RemoveImage(id); err != nil {
					dv.imagesView.Log(fmt.Sprintf("[red]Failed to remove image: %v", err))
				} else {
					dv.imagesView.Log(fmt.Sprintf("[yellow]Image %s removed", name))
					dv.refresh()
				}
			}
			dv.app.SetFocus(dv.imagesView.GetTable())
		},
	)
}

func (dv *DockerView) pullImage() {
	ui.ShowCompactStyledInputModal(
		dv.pages,
		dv.app,
		"Pull Image",
		"Image Name",
		"nginx:latest",
		40,
		nil,
		func(imageName string, cancelled bool) {
			if cancelled || imageName == "" {
				dv.app.SetFocus(dv.imagesView.GetTable())
				return
			}

			dv.imagesView.Log(fmt.Sprintf("[yellow]Pulling image %s...", imageName))

			go func() {
				if err := dv.dockerClient.PullImage(imageName); err != nil {
					dv.app.QueueUpdateDraw(func() {
						dv.imagesView.Log(fmt.Sprintf("[red]Failed to pull image: %v", err))
					})
				} else {
					dv.app.QueueUpdateDraw(func() {
						dv.imagesView.Log(fmt.Sprintf("[green]Image %s pulled successfully", imageName))
						dv.refresh()
					})
				}
			}()

			dv.app.SetFocus(dv.imagesView.GetTable())
		},
	)
}

func (dv *DockerView) showImageHistory() {
	id, ok := dv.getSelectedImageID()
	if !ok {
		dv.imagesView.Log("[yellow]No image selected")
		return
	}

	name := dv.getSelectedImageName()
	dv.imagesView.Log(fmt.Sprintf("[yellow]Getting history for %s...", name))

	history, err := dv.dockerClient.GetImageHistory(id)
	if err != nil {
		dv.imagesView.Log(fmt.Sprintf("[red]Failed to get image history: %v", err))
		return
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		fmt.Sprintf("History: %s", name),
		history,
		func() {
			dv.app.SetFocus(dv.imagesView.GetTable())
		},
	)
}

func (dv *DockerView) runImageAsContainer() {
	_, ok := dv.getSelectedImageID()
	if !ok {
		dv.imagesView.Log("[yellow]No image selected")
		return
	}

	imageName := dv.getSelectedImageName()

	ui.ShowCompactStyledInputModal(
		dv.pages,
		dv.app,
		"Run Container",
		"Container Name",
		"",
		30,
		nil,
		func(containerName string, cancelled bool) {
			if cancelled {
				dv.app.SetFocus(dv.imagesView.GetTable())
				return
			}

			dv.imagesView.Log(fmt.Sprintf("[yellow]Creating container from %s...", imageName))

			containerID, err := dv.dockerClient.CreateContainer(imageName, containerName)
			if err != nil {
				dv.imagesView.Log(fmt.Sprintf("[red]Failed to create container: %v", err))
				dv.app.SetFocus(dv.imagesView.GetTable())
				return
			}

			dv.imagesView.Log(fmt.Sprintf("[green]Container created: %s", containerID[:12]))

			// Start the container
			if err := dv.dockerClient.StartContainer(containerID); err != nil {
				dv.imagesView.Log(fmt.Sprintf("[red]Failed to start container: %v", err))
			} else {
				dv.imagesView.Log(fmt.Sprintf("[green]Container started: %s", containerID[:12]))
			}

			dv.app.SetFocus(dv.imagesView.GetTable())
		},
	)
}

func (dv *DockerView) showImageDetails() {
	id, ok := dv.getSelectedImageID()
	if !ok {
		return
	}

	name := dv.getSelectedImageName()
	dv.imagesView.Log(fmt.Sprintf("[yellow]Inspecting image %s...", name))

	inspect, err := dv.dockerClient.InspectImage(id)
	if err != nil {
		dv.imagesView.Log(fmt.Sprintf("[red]Failed to inspect image: %v", err))
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Image: %s[white]\n\n", name))
	details.WriteString(fmt.Sprintf("[green]ID:[white] %s\n", inspect.ID))
	details.WriteString(fmt.Sprintf("[green]Created:[white] %s\n", inspect.Created))
	details.WriteString(fmt.Sprintf("[green]Size:[white] %s\n", inspect.Size))
	details.WriteString(fmt.Sprintf("[green]Architecture:[white] %s\n", inspect.Architecture))
	details.WriteString(fmt.Sprintf("[green]OS:[white] %s\n", inspect.OS))
	details.WriteString(fmt.Sprintf("[green]Docker Version:[white] %s\n", inspect.DockerVersion))

	if len(inspect.RepoTags) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Tags:[white]\n"))
		for _, tag := range inspect.RepoTags {
			details.WriteString(fmt.Sprintf("  %s\n", tag))
		}
	}

	if len(inspect.RepoDigests) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Digests:[white]\n"))
		for _, digest := range inspect.RepoDigests {
			details.WriteString(fmt.Sprintf("  %s\n", digest))
		}
	}

	if len(inspect.ExposedPorts) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Exposed Ports:[white]\n"))
		for _, port := range inspect.ExposedPorts {
			details.WriteString(fmt.Sprintf("  %s\n", port))
		}
	}

	if len(inspect.Env) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Environment:[white]\n"))
		for _, env := range inspect.Env {
			details.WriteString(fmt.Sprintf("  %s\n", env))
		}
	}

	ui.ShowInfoModal(
		dv.pages,
		dv.app,
		fmt.Sprintf("Image: %s", name),
		details.String(),
		func() {
			dv.app.SetFocus(dv.imagesView.GetTable())
		},
	)
}
