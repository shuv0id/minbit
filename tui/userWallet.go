package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var UserWallet = GenerateWallet()

const (
	recipentAddr = iota
	amount
	send
)

type usrWalletMdl struct {
	balance    int
	txtInputs  []textinput.Model
	focusIndex int
	err        error
}

func InitialUsrWallet() tea.Model {
	usrWallet := usrWalletMdl{}
	var inputs []textinput.Model = make([]textinput.Model, 2)

	// Recipient Address Input
	inputs[recipentAddr] = textinput.New()
	inputs[recipentAddr].CharLimit = 58
	inputs[recipentAddr].Prompt = "-> "
	inputs[recipentAddr].Placeholder = "Address of the wallet to send BTC to"
	inputs[recipentAddr].Width = 60
	inputs[recipentAddr].Validate = addressValidator
	inputs[recipentAddr].Focus()

	// Amount Input
	inputs[amount] = textinput.New()
	inputs[amount].Width = 30
	inputs[amount].Prompt = "-> "
	inputs[amount].Placeholder = "Amount of BTC to send"
	inputs[amount].Validate = amountValidator

	usrWallet.txtInputs = inputs
	return usrWallet
}

func (usrWallet usrWalletMdl) Init() tea.Cmd {
	return nil
}

func (usrWallet usrWalletMdl) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:

		switch msg.String() {
		case "esc", "ctrl+c":
			return usrWallet, tea.Quit
		case "down":
			if usrWallet.focusIndex < len(usrWallet.txtInputs) { // Allow focus to move to "Send"
				usrWallet.focusIndex++
			}
		case "up":
			if usrWallet.focusIndex > 0 {
				usrWallet.focusIndex--
			}
		case "enter":
			if usrWallet.focusIndex == send {
				if usrWallet.err != nil {
					sendBTC(usrWallet.txtInputs[recipentAddr].Value(), amount)
				}
			}
		}

		for i := 0; i < len(usrWallet.txtInputs); i++ {
			if i == usrWallet.focusIndex {
				usrWallet.txtInputs[i].Focus()
			} else {
				usrWallet.txtInputs[i].Blur()
			}
			usrWallet.txtInputs[i], cmd = usrWallet.txtInputs[i].Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		winWidth = msg.Width
		winHeight = msg.Height
	}

	var errMsg error
	if usrWallet.focusIndex != recipentAddr && usrWallet.txtInputs[recipentAddr].Err != nil {
		errMsg = usrWallet.txtInputs[recipentAddr].Err
	}
	if usrWallet.focusIndex != amount && usrWallet.txtInputs[amount].Err != nil {
		errMsg = usrWallet.txtInputs[amount].Err
	}
	usrWallet.err = errMsg

	return usrWallet, tea.Batch(cmds...)
}

func (usrWallet usrWalletMdl) View() string {
	sendButton := " Send "
	if usrWallet.focusIndex == send {
		sendButton = buttonFocusedStyle.Render(sendButton)
	} else {
		sendButton = buttonStyle.Render(sendButton)
	}

	errMsg := ""
	if usrWallet.err != nil {
		errMsg = errorStyle.Render(usrWallet.err.Error())
	}

	content := fmt.Sprintf(
		`~~ Send btc~~
%s

%s
%s

%s
%s

%s

%s
`,
		errMsg,
		inputStyle.Render("Address"),
		usrWallet.txtInputs[recipentAddr].View(),
		inputStyle.Render("Amount"),
		usrWallet.txtInputs[amount].View(),
		sendButton,
		helpStyle.Render("Press Esc to quit."),
	)

	return Centered(usrWallet, content, winWidth, winHeight)
}
