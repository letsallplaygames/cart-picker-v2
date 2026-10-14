package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"pickcart/internal/config"
)

const (
	appID           = "com.allplay.pickcart"
	prefCartNumber  = "cart_number"
	prefProfileName = "profile_name"
)

type LaunchSettings struct {
	CartNumber      int
	ProfileName     string
	CartFromFlag    bool
	ProfileFromFlag bool
}

func RunLauncher(defaults LaunchSettings, start func(LaunchSettings) (*App, error)) {
	if start == nil {
		return
	}

	fa := fyneapp.NewWithID(appID)
	w := fa.NewWindow("PickCart")

	settings := resolveLaunchSettings(fa.Preferences(), defaults)

	cartEntry := widget.NewEntry()
	cartEntry.SetText(strconv.Itoa(settings.CartNumber))
	cartEntry.SetPlaceHolder("Cart number")

	displayNames, displayByName := profileSelectOptions()
	profileSelect := widget.NewSelect(displayNames, nil)
	if display, ok := displayByName[settings.ProfileName]; ok {
		profileSelect.SetSelected(display)
	} else if len(displayNames) > 0 {
		profileSelect.SetSelected(displayNames[0])
	}

	status := widget.NewLabel("")
	var startBtn *widget.Button
	starting := false

	doStart := func() {
		if starting {
			return
		}

		cartNumber, err := parseCartNumber(cartEntry.Text)
		if err != nil {
			status.SetText(err.Error())
			return
		}
		profileName, err := profileNameFromDisplay(profileSelect.Selected)
		if err != nil {
			status.SetText(err.Error())
			return
		}

		starting = true
		if startBtn != nil {
			startBtn.Disable()
		}
		status.SetText("Starting...")

		next := LaunchSettings{CartNumber: cartNumber, ProfileName: profileName}
		application, err := start(next)
		if err != nil {
			starting = false
			if startBtn != nil {
				startBtn.Enable()
			}
			status.SetText("")
			dialog.ShowError(err, w)
			return
		}

		fa.Preferences().SetInt(prefCartNumber, next.CartNumber)
		fa.Preferences().SetString(prefProfileName, next.ProfileName)
		application.attach(fa, w)
		application.startLoading()
	}

	startBtn = widget.NewButton("Start PickCart", doStart)
	startBtn.Importance = widget.HighImportance
	cartEntry.OnSubmitted = func(string) { doStart() }

	title := widget.NewLabel("PickCart")
	title.Alignment = fyne.TextAlignCenter
	title.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := widget.NewLabel("Choose the cart number and profile, then start.")
	subtitle.Alignment = fyne.TextAlignCenter
	subtitle.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem("Cart number", cartEntry),
		widget.NewFormItem("Cart profile", profileSelect),
	)

	w.SetContent(container.NewPadded(container.NewVBox(title, subtitle, form, startBtn, status)))
	w.Resize(fyne.NewSize(480, 320))
	w.CenterOnScreen()
	w.ShowAndRun()
}

func resolveLaunchSettings(prefs fyne.Preferences, defaults LaunchSettings) LaunchSettings {
	settings := defaults
	if settings.CartNumber < 1 {
		settings.CartNumber = 1
	}
	if strings.TrimSpace(settings.ProfileName) == "" {
		settings.ProfileName = "standard"
	}
	if prefs != nil {
		if !settings.CartFromFlag {
			settings.CartNumber = prefs.IntWithFallback(prefCartNumber, settings.CartNumber)
		}
		if !settings.ProfileFromFlag {
			settings.ProfileName = prefs.StringWithFallback(prefProfileName, settings.ProfileName)
		}
	}
	if _, err := config.GetCartProfile(settings.ProfileName); err != nil {
		settings.ProfileName = "standard"
	}
	if settings.CartNumber < 1 {
		settings.CartNumber = 1
	}
	return settings
}

func parseCartNumber(value string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("enter a cart number of 1 or greater")
	}
	return n, nil
}

func profileSelectOptions() ([]string, map[string]string) {
	names := config.AvailableProfiles()
	displayNames := make([]string, 0, len(names))
	displayByName := make(map[string]string, len(names))
	for _, name := range names {
		profile, err := config.GetCartProfile(name)
		if err != nil {
			continue
		}
		display := profile.DisplayName
		if display == "" {
			display = profile.Name
		}
		displayNames = append(displayNames, display)
		displayByName[profile.Name] = display
	}
	return displayNames, displayByName
}

func profileNameFromDisplay(display string) (string, error) {
	display = strings.TrimSpace(display)
	if display == "" {
		return "", fmt.Errorf("select a cart profile")
	}
	for _, name := range config.AvailableProfiles() {
		profile, err := config.GetCartProfile(name)
		if err != nil {
			continue
		}
		if profile.DisplayName == display || profile.Name == display {
			return profile.Name, nil
		}
	}
	return "", fmt.Errorf("unknown cart profile %q", display)
}
