package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/ui"
)

// AccountView handles the account management view
type AccountView struct {
	app       *tview.Application
	pages     *tview.Pages
	cores     *ui.Cores
	apiClient *ArgoAPIClient
	accounts  []Account
}

// NewAccountView creates a new account view
func NewAccountView(app *tview.Application, pages *tview.Pages, cores *ui.Cores, apiClient *ArgoAPIClient) *AccountView {
	return &AccountView{
		app:       app,
		pages:     pages,
		cores:     cores,
		apiClient: apiClient,
		accounts:  []Account{},
	}
}

// fetchAccounts gets the list of accounts from ArgoCD
func (v *AccountView) fetchAccounts() ([][]string, error) {
	// Check if connected
	if !v.apiClient.IsConnected {
		return [][]string{
			{"Not connected to ArgoCD", "", "", ""},
		}, nil
	}

	// Fetch accounts
	accounts, err := v.apiClient.GetAccounts()
	if err != nil {
		return [][]string{
			{fmt.Sprintf("Error: %v", err), "", "", ""},
		}, err
	}

	// Store for later use
	v.accounts = accounts

	// Format data for display
	result := [][]string{}
	for _, account := range accounts {
		// Format capabilities
		capabilities := strings.Join(account.Capabilities, ", ")
		if capabilities == "" {
			capabilities = "None"
		}

		// Format enabled status
		enabled := "No"
		if account.Enabled {
			enabled = "Yes"
		}

		// Format token count
		tokenCount := fmt.Sprintf("%d", len(account.Tokens))

		result = append(result, []string{
			account.Name,
			capabilities,
			enabled,
			tokenCount,
		})
	}

	if len(result) == 0 {
		return [][]string{
			{"No accounts found", "", "", ""},
		}, nil
	}

	return result, nil
}

// getSelectedAccount returns the currently selected account
func (v *AccountView) getSelectedAccount() *Account {
	row := v.cores.GetSelectedRow()
	if row < 0 || row >= len(v.accounts) {
		return nil
	}
	return &v.accounts[row]
}

// showAccountDetailsModal displays details of the selected account
func (v *AccountView) showAccountDetailsModal() {
	// Get selected account
	account := v.getSelectedAccount()
	if account == nil {
		v.cores.Log("[red]Please select an account to view")
		return
	}

	// Get fresh account data
	var err error
	account, err = v.apiClient.GetAccount(account.Name)
	if err != nil {
		v.cores.Log(fmt.Sprintf("[red]Failed to get account details: %v", err))
		return
	}

	// Format capabilities
	capabilities := "None"
	if len(account.Capabilities) > 0 {
		capabilities = strings.Join(account.Capabilities, ", ")
	}

	// Format tokens
	tokensText := "None"
	if len(account.Tokens) > 0 {
		tokenList := []string{}
		for _, token := range account.Tokens {
			tokenInfo := fmt.Sprintf("- ID: %s\n  Issued: %s\n  Expires: %s",
				token.ID, token.IssuedAt, token.ExpiresAt)
			tokenList = append(tokenList, tokenInfo)
		}
		tokensText = strings.Join(tokenList, "\n\n")
	}

	// Create content for modal
	content := fmt.Sprintf(`[yellow]Account: %s[white]

[green]Enabled:[white] %t

[green]Capabilities:[white]
%s

[green]Tokens:[white]
%s
`,
		account.Name,
		account.Enabled,
		capabilities,
		tokensText,
	)

	// Show modal
	ui.ShowInfoModal(
		v.pages,
		v.app,
		fmt.Sprintf("Account Details: %s", account.Name),
		content,
		func() {
			v.app.SetFocus(v.cores.GetTable())
		},
	)
}

// showCreateAccountModal displays a modal to create a new account
func (v *AccountView) showCreateAccountModal() {
	// Create form
	form := tview.NewForm()
	form.AddInputField("Account Name:", "", 20, nil, nil)
	form.AddCheckbox("Enabled:", true, nil)
	form.AddInputField("Capabilities (comma-separated):", "login", 40, nil, nil)
	form.AddPasswordField("Password:", "", 30, '*', nil)

	// Add buttons
	form.AddButton("Create", func() {
		// Get form values
		name := form.GetFormItem(0).(*tview.InputField).GetText()
		enabled := form.GetFormItem(1).(*tview.Checkbox).IsChecked()
		capabilitiesInput := form.GetFormItem(2).(*tview.InputField).GetText()
		password := form.GetFormItem(3).(*tview.InputField).GetText()

		// Validate inputs
		if name == "" {
			v.cores.Log("[red]Account name is required")
			return
		}

		if password == "" {
			v.cores.Log("[red]Password is required")
			return
		}

		// Parse capabilities
		capabilities := []string{}
		if capabilitiesInput != "" {
			capabilities = strings.Split(capabilitiesInput, ",")
			for i, cap := range capabilities {
				capabilities[i] = strings.TrimSpace(cap)
			}
		}

		// Create account object
		account := Account{
			Name:         name,
			Enabled:      enabled,
			Capabilities: capabilities,
			Tokens:       []Token{},
		}

		// Show progress
		pm := ui.ShowProgressModal(
			v.pages, v.app, "Creating account", 100, true,
			nil, true,
		)

		// Create account
		safeGo(func() {
			err := v.apiClient.CreateAccount(account, password)
			if err != nil {
				v.app.QueueUpdateDraw(func() {
					pm.Close()
					v.cores.Log(fmt.Sprintf("[red]Failed to create account: %v", err))
				})
				return
			}

			v.app.QueueUpdateDraw(func() {
				pm.Close()
				v.pages.RemovePage("create-account-modal")
				v.cores.Log(fmt.Sprintf("[green]Created account: %s", name))
				v.cores.RefreshData()
			})
		})
	})

	form.AddButton("Cancel", func() {
		v.pages.RemovePage("create-account-modal")
	})

	// Style the form
	form.SetBorder(true)
	form.SetTitle(" Create Account ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorBlue)
	form.SetTitleColor(tcell.ColorYellow)
	form.SetBackgroundColor(tcell.ColorDefault)

	// Create centered modal
	width := 60
	height := 12

	centerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Add to pages
	v.pages.AddPage("create-account-modal", centerFlex, true, true)

	// Set focus
	v.app.SetFocus(form.GetFormItem(0))

	// Add ESC handler
	ui.RemovePage(v.pages, v.app, "create-account-modal", nil)
}

// showDeleteAccountModal displays a modal to delete an account
func (v *AccountView) showDeleteAccountModal() {
	// Get selected account
	account := v.getSelectedAccount()
	if account == nil {
		v.cores.Log("[red]Please select an account to delete")
		return
	}

	// Show confirmation dialog
	ui.ShowStandardConfirmationModal(
		v.pages,
		v.app,
		"Delete Account",
		fmt.Sprintf("Are you sure you want to delete the account '%s'?", account.Name),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			// Show progress
			pm := ui.ShowProgressModal(
				v.pages, v.app, "Deleting account", 100, true,
				nil, true,
			)

			// Delete account
			accountName := account.Name
			safeGo(func() {
				err := v.apiClient.DeleteAccount(accountName)
				if err != nil {
					v.app.QueueUpdateDraw(func() {
						pm.Close()
						v.cores.Log(fmt.Sprintf("[red]Failed to delete account: %v", err))
					})
					return
				}

				v.app.QueueUpdateDraw(func() {
					pm.Close()
					v.cores.Log(fmt.Sprintf("[green]Deleted account: %s", accountName))
					v.cores.RefreshData()
				})
			})
		},
	)
}

// showCreateTokenModal displays a modal to create a token for an account
func (v *AccountView) showCreateTokenModal() {
	// Get selected account
	account := v.getSelectedAccount()
	if account == nil {
		v.cores.Log("[red]Please select an account to create a token for")
		return
	}

	// Create form
	form := tview.NewForm()
	form.AddInputField("Account:", account.Name, 20, nil, nil).
		SetFieldBackgroundColor(tcell.ColorDarkGray)
	form.AddInputField("Expiration (hours):", "24", 20, nil, nil)

	// Add buttons
	form.AddButton("Create", func() {
		// Get form values
		expirationText := form.GetFormItem(1).(*tview.InputField).GetText()
		expiration := 24 // Default to 24 hours
		fmt.Sscanf(expirationText, "%d", &expiration)

		// Show progress
		pm := ui.ShowProgressModal(
			v.pages, v.app, "Creating token", 100, true,
			nil, true,
		)

		// Create token
		accountName := account.Name
		safeGo(func() {
			token, err := v.apiClient.CreateToken(accountName, expiration)
			if err != nil {
				v.app.QueueUpdateDraw(func() {
					pm.Close()
					v.cores.Log(fmt.Sprintf("[red]Failed to create token: %v", err))
				})
				return
			}

			v.app.QueueUpdateDraw(func() {
				pm.Close()
				v.pages.RemovePage("create-token-modal")

				// Display the token information
				content := fmt.Sprintf(`[yellow]Token Created[white]

[green]Account:[white] %s

[green]Token:[white] 
%s

[green]Issued:[white] %s

[green]Expires:[white] %s

[red]IMPORTANT: Save this token now. You will not be able to view it again.[white]
`,
					accountName,
					token.Token,
					token.IssuedAt,
					token.ExpiresAt,
				)

				ui.ShowInfoModal(
					v.pages,
					v.app,
					"Token Created",
					content,
					func() {
						v.cores.RefreshData()
						v.app.SetFocus(v.cores.GetTable())
					},
				)
			})
		})
	})

	form.AddButton("Cancel", func() {
		v.pages.RemovePage("create-token-modal")
	})

	// Style the form
	form.SetBorder(true)
	form.SetTitle(" Create Token ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorBlue)
	form.SetTitleColor(tcell.ColorYellow)
	form.SetBackgroundColor(tcell.ColorDefault)

	// Create centered modal
	width := 50
	height := 10

	centerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Add to pages
	v.pages.AddPage("create-token-modal", centerFlex, true, true)

	// Set focus
	v.app.SetFocus(form.GetFormItem(1))

	// Add ESC handler
	ui.RemovePage(v.pages, v.app, "create-token-modal", nil)
}

// SetupTableHandlers sets up the handlers for table events
func (v *AccountView) SetupTableHandlers() {
	if v.cores == nil {
		Debug("Cannot set up table handlers: cores is nil")
		return
	}

	// Set up selected row handler
	v.cores.SetRowSelectedCallback(v.onAccountSelected)

	Debug("Set up table handlers for account view")
}

// onAccountSelected is called when an account is selected in the table
func (v *AccountView) onAccountSelected(row int) {
	// Make sure the row is valid
	if v.cores == nil {
		Debug("Cores reference is nil in onAccountSelected")
		return
	}

	Debug("onAccountSelected called with row %d", row)
	Debug("Accounts length: %d", len(v.accounts))

	if row < 0 || row >= len(v.accounts) {
		Debug("Invalid row %d (not in range 0-%d)", row, len(v.accounts)-1)
		return
	}

	// Get the selected account
	account := v.accounts[row]
	Debug("Selected account: %s at row %d", account.Name, row)

	// Update status bar or other UI elements with selected account info
	if account.Name != "" {
		// Format enabled status
		enabledStatus := "disabled"
		if account.Enabled {
			enabledStatus = "enabled"
		}

		// Format token count
		tokenCount := len(account.Tokens)
		tokenText := fmt.Sprintf("%d token%s", tokenCount,
			map[bool]string{true: "s", false: ""}[tokenCount != 1])

		// Format capabilities count
		capCount := len(account.Capabilities)
		capText := fmt.Sprintf("%d capabilit%s", capCount,
			map[bool]string{true: "ies", false: "y"}[capCount != 1])

		v.cores.Log(fmt.Sprintf("[blue]Selected account: %s (%s, %s, %s)",
			account.Name, enabledStatus, tokenText, capText))
	}
}
