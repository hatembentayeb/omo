package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (gv *GitView) newTagsView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Git Tags")
	cores.SetTableHeaders([]string{"Tag", "Type", "Date", "Message"})
	cores.SetRefreshCallback(gv.refreshTagsData)
	cores.SetSelectionKey("Tag")

	// Navigation key bindings
	cores.AddKeyBinding("G", "Repos", gv.showRepos)
	cores.AddKeyBinding("S", "Status", gv.showStatus)
	cores.AddKeyBinding("L", "Commits", gv.showCommits)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("M", "Remotes", gv.showRemotes)
	cores.AddKeyBinding("T", "Tags", gv.showTags)
	cores.AddKeyBinding("H", "Stash", gv.showStash)

	// Tag action key bindings
	cores.AddKeyBinding("N", "New Tag", gv.createTag)
	cores.AddKeyBinding("D", "Delete", gv.deleteTag)
	cores.AddKeyBinding("C", "Checkout", gv.checkoutTag)
	cores.AddKeyBinding("P", "Push", gv.pushTag)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.RegisterHandlers()
	return cores
}

func (gv *GitView) refreshTagsData() ([][]string, error) {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		return [][]string{{"No repository selected", "", "", ""}}, nil
	}

	tags, err := gv.gitClient.GetTagsInfo(repo.Path)
	if err != nil {
		return [][]string{{fmt.Sprintf("Error: %v", err), "", "", ""}}, err
	}

	if len(tags) == 0 {
		return [][]string{{"No tags", "", "", ""}}, nil
	}

	rows := make([][]string, len(tags))
	for i, tag := range tags {
		tagType := "lightweight"
		if tag.IsAnnotated {
			tagType = "[green]annotated[white]"
		}

		rows[i] = []string{
			fmt.Sprintf("[yellow]%s[white]", tag.Name),
			tagType,
			tag.Date,
			truncateString(tag.Message, 40),
		}
	}

	if gv.tagsView != nil {
		gv.tagsView.SetInfoText(fmt.Sprintf("[green]Git Tags[white]\nRepo: %s\nTags: %d",
			repo.Name, len(tags)))
	}

	return rows, nil
}

func (gv *GitView) getSelectedTagName() (string, bool) {
	if gv.tagsView == nil {
		return "", false
	}
	row := gv.tagsView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	name := stripColorCodes(row[0])
	return name, name != "" && name != "No repository selected" && name != "No tags"
}

func (gv *GitView) createTag() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.tagsView.Log("[yellow]No repository selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Create Tag",
		"Tag Name",
		"",
		30,
		nil,
		func(tagName string, cancelled bool) {
			if cancelled || tagName == "" {
				gv.app.SetFocus(gv.tagsView.GetTable())
				return
			}

			ui.ShowCompactStyledInputModal(
				gv.pages,
				gv.app,
				"Create Tag",
				"Message (empty for lightweight)",
				"",
				50,
				nil,
				func(message string, cancelled bool) {
					if cancelled {
						gv.app.SetFocus(gv.tagsView.GetTable())
						return
					}

					var err error
					if message != "" {
						err = gv.gitClient.CreateAnnotatedTag(repo.Path, tagName, message)
					} else {
						err = gv.gitClient.CreateLightweightTag(repo.Path, tagName)
					}

					if err != nil {
						gv.tagsView.Log(fmt.Sprintf("[red]Failed to create tag: %v", err))
					} else {
						gv.tagsView.Log(fmt.Sprintf("[green]Created tag %s", tagName))
						gv.refresh()
					}
					gv.app.SetFocus(gv.tagsView.GetTable())
				},
			)
		},
	)
}

func (gv *GitView) deleteTag() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.tagsView.Log("[yellow]No repository selected")
		return
	}

	tagName, ok := gv.getSelectedTagName()
	if !ok {
		gv.tagsView.Log("[yellow]No tag selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Delete Tag",
		fmt.Sprintf("Delete tag [red]%s[white]?", tagName),
		func(confirmed bool) {
			if confirmed {
				if err := gv.gitClient.DeleteTag(repo.Path, tagName); err != nil {
					gv.tagsView.Log(fmt.Sprintf("[red]Delete failed: %v", err))
				} else {
					gv.tagsView.Log(fmt.Sprintf("[green]Deleted tag %s", tagName))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.tagsView.GetTable())
		},
	)
}

func (gv *GitView) checkoutTag() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.tagsView.Log("[yellow]No repository selected")
		return
	}

	tagName, ok := gv.getSelectedTagName()
	if !ok {
		gv.tagsView.Log("[yellow]No tag selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Checkout Tag",
		fmt.Sprintf("Checkout tag [yellow]%s[white]?\nThis will put you in detached HEAD state.", tagName),
		func(confirmed bool) {
			if confirmed {
				if err := gv.gitClient.Checkout(repo.Path, tagName); err != nil {
					gv.tagsView.Log(fmt.Sprintf("[red]Checkout failed: %v", err))
				} else {
					gv.tagsView.Log(fmt.Sprintf("[green]Checked out tag %s", tagName))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.tagsView.GetTable())
		},
	)
}

func (gv *GitView) pushTag() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.tagsView.Log("[yellow]No repository selected")
		return
	}

	tagName, ok := gv.getSelectedTagName()
	if !ok {
		gv.tagsView.Log("[yellow]No tag selected")
		return
	}

	gv.tagsView.Log(fmt.Sprintf("[yellow]Pushing tag %s...", tagName))

	go func() {
		if err := gv.gitClient.PushTag(repo.Path, tagName); err != nil {
			gv.app.QueueUpdateDraw(func() {
				gv.tagsView.Log(fmt.Sprintf("[red]Push failed: %v", err))
			})
		} else {
			gv.app.QueueUpdateDraw(func() {
				gv.tagsView.Log(fmt.Sprintf("[green]Pushed tag %s", tagName))
			})
		}
	}()
}
