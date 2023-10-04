package task

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/raxigan/pcfy-my-mac/cmd/common"
	"github.com/raxigan/pcfy-my-mac/cmd/install"
	"github.com/raxigan/pcfy-my-mac/cmd/param"
	"path/filepath"
	"strings"
)

type Task struct {
	Name    string
	Execute func(i install.Installation) error
}

func DownloadDependencies() Task {
	return Task{
		Name: "Install dependencies",
		Execute: func(i install.Installation) error {

			var notInstalled []string
			var commands []string

			all := []Dependency{JqDependency(), KarabinerDependency(), AltTabDependency(), RectangleDependency()}

			for _, d := range all {
				if !i.Exists(d.command) {
					notInstalled = append(notInstalled, d.name)
					commands = append(commands, d.installCommand)
				}
			}

			if len(notInstalled) > 0 {
				installApp := false
				prompt := &survey.Confirm{
					Message: fmt.Sprintf("The following dependencies will be installed: %s. Do you agree?", strings.Join(notInstalled, ", ")),
				}
				common.HandleInterrupt(
					survey.AskOne(prompt, &installApp),
				)

				if !installApp {
					fmt.Printf("Qutting...")
					i.Exit(0)
				}

				for _, c := range commands {
					i.Run(c)
				}
			}

			return nil

		},
	}
}

func CloseKarabiner() Task {
	return Task{
		Name:    "Close Karabiner",
		Execute: func(i install.Installation) error { i.Run("killall Karabiner-Elements"); return nil },
	}
}

func BackupKarabinerConfig() Task {
	return Task{
		Name: "Do karabiner config backup",
		Execute: func(i install.Installation) error {
			original := i.KarabinerConfigFile()
			backupDest := i.KarabinerConfigBackupFile(i.InstallationTime)
			common.CopyFile(original, backupDest)
			return nil
		},
	}
}

func DeleteExistingKarabinerProfile() Task {
	return Task{
		Name: "Delete existing Karabiner profile",
		Execute: func(i install.Installation) error {
			deleteProfileJqCmd := fmt.Sprintf("jq --arg PROFILE_NAME \"%s\" 'del(.profiles[] | select(.name == \"%s\"))' %s >tmp && mv tmp %s", i.ProfileName, i.ProfileName, i.KarabinerConfigFile(), i.KarabinerConfigFile())
			i.Run(deleteProfileJqCmd)
			return nil
		},
	}
}

func CreateKarabinerProfile() Task {
	return Task{
		Name: "Delete existing Karabiner profile",
		Execute: func(i install.Installation) error {
			common.CopyFileFromEmbedFS("karabiner/karabiner-profile.json", "tmp")
			addProfileJqCmd := fmt.Sprintf("jq '.profiles += $profile' %s --slurpfile profile tmp --indent 4 >INPUT.tmp && mv INPUT.tmp %s && rm tmp", i.KarabinerConfigFile(), i.KarabinerConfigFile())
			i.Run(addProfileJqCmd)
			return nil
		},
	}
}

func NameKarabinerProfile() Task {
	return Task{
		Name: "Delete existing Karabiner profile",
		Execute: func(i install.Installation) error {
			common.CopyFileFromEmbedFS("karabiner/karabiner-profile.json", "tmp")
			addProfileJqCmd := fmt.Sprintf("jq '.profiles |= map(if .name == \"_PROFILE_NAME_\" then .name = \"%s\" else . end)' %s > tmp && mv tmp %s", i.ProfileName, i.KarabinerConfigFile(), i.KarabinerConfigFile())
			i.Run(addProfileJqCmd)
			return nil
		},
	}
}

func UnselectOtherKarabinerProfiles() Task {
	return Task{
		Name: "Unselect other Karabiner profiles",
		Execute: func(i install.Installation) error {
			unselectJqCmd := fmt.Sprintf("jq '.profiles |= map(if .name != \"%s\" then .selected = false else . end)' %s > tmp && mv tmp %s", i.ProfileName, i.KarabinerConfigFile(), i.KarabinerConfigFile())
			i.Run(unselectJqCmd)
			return nil
		},
	}
}

func ApplyMainKarabinerRules() Task {
	return Task{
		Name: "Apply main Karabiner rules",
		Execute: func(i install.Installation) error {
			ApplyRules(i, "main.json")
			ApplyRules(i, "finder.json")
			return nil
		},
	}
}

func ApplyAppLauncherRules() Task {
	return Task{
		Name: "Apply app launcher rules",
		Execute: func(i install.Installation) error {
			switch strings.ToLower(i.AppLauncher) {
			case "spotlight":
				ApplyRules(i, "spotlight.json")
			case "launchpad":
				ApplyRules(i, "launchpad.json")
			case "alfred":
				{
					if i.Exists("Alfred 4.app") || i.Exists("Alfred 5.app") {

						ApplyRules(i, "alfred.json")

						dirs, err := common.FindMatchingDirs(i.ApplicationSupportDir()+"/Alfred/Alfred.alfredpreferences/preferences/local", "", "hotkey", "prefs.plist")

						if err != nil {
							return err
						}

						for _, e := range dirs {
							common.CopyFileFromEmbedFS("alfred/prefs.plist", e)
						}
					} else {
						common.PrintColored(common.Yellow, fmt.Sprintf("Alfred app not found. Skipping..."))
					}
				}
			}

			return nil
		},
	}
}

func ApplyKeyboardLayoutRules() Task {
	return Task{
		Name: "Apply keyboard layout rules",
		Execute: func(i install.Installation) error {
			switch strings.ToLower(i.KeyboardLayout) {
			case "mac":
				jq := fmt.Sprintf("jq --arg PROFILE_NAME \"%s\" '.profiles |= map(if .name == \"%s\" then walk(if type == \"object\" and .conditions then del(.conditions[] | select(.identifiers[]?.is_built_in_keyboard)) else . end) else . end)' %s --indent 4 >tmp && mv tmp %s", i.ProfileName, i.ProfileName, i.KarabinerConfigFile(), i.KarabinerConfigFile())
				i.Run(jq)
			}
			return nil
		},
	}
}

func ApplyTerminalRules() Task {
	return Task{
		Name: "Apply terminal rules",
		Execute: func(i install.Installation) error {
			switch strings.ToLower(i.Terminal) {
			case "default":
				ApplyRules(i, "apple-terminal.json")
			case "iterm":
				if i.Exists("iTerm.app") {
					ApplyRules(i, "iterm.json")
				} else {
					common.PrintColored(common.Yellow, fmt.Sprintf("iTerm app not found. Skipping..."))
				}
			case "warp":
				{
					if i.Exists("Warp.app") {
						ApplyRules(i, "warp.json")
					} else {
						common.PrintColored(common.Yellow, fmt.Sprintf("Warp app not found. Skipping..."))
					}
				}
			}

			return nil
		},
	}
}

func ReformatKarabinerConfigFile() Task {
	return Task{
		Name: "Reformat Karabiner config file",
		Execute: func(i install.Installation) error {
			i.Run(fmt.Sprintf("jq '.' %s > tmp && mv tmp %s", i.KarabinerConfigFile(), i.KarabinerConfigFile()))
			return nil
		},
	}
}

func OpenKarabiner() Task {
	return Task{
		Name: "Open Karabiner-Elements.app",
		Execute: func(i install.Installation) error {
			i.Run("open -a Karabiner-Elements")
			return nil
		},
	}
}

func CopyIdeKeymaps() Task {
	return Task{
		Name: "Install IDE keymaps",
		Execute: func(i install.Installation) error {
			for _, ide := range i.Ides {
				name, _ := param.IdeKeymapByFullName(ide)
				InstallIdeKeymap(i, name)
			}
			return nil
		},
	}
}

func CloseRectangle() Task {
	return Task{
		Name: "Close rectangle",
		Execute: func(i install.Installation) error {
			i.Run("killall Rectangle")
			return nil
		},
	}
}

func CopyRectanglePreferences() Task {
	return Task{
		Name: "Install Rectangle preferences",
		Execute: func(i install.Installation) error {
			rectanglePlist := filepath.Join(i.PreferencesDir(), "com.knollsoft.Rectangle.plist")
			common.CopyFileFromEmbedFS("rectangle/Settings.xml", rectanglePlist)

			plutilCmdRectangle := fmt.Sprintf("plutil -convert binary1 %s", rectanglePlist)
			i.Run(plutilCmdRectangle)
			i.Run("defaults read com.knollsoft.Rectangle.plist")
			return nil
		},
	}
}

func OpenRectangle() Task {
	return Task{
		Name: "Open Rectangle.app",
		Execute: func(i install.Installation) error {
			i.Run("open -a Rectangle")
			return nil
		},
	}
}

func CloseAltTab() Task {
	return Task{
		Name: "Close AtlTab.app",
		Execute: func(i install.Installation) error {
			i.Run("killall AltTab")
			return nil
		},
	}
}

func InstallAltTabPreferences() Task {
	return Task{
		Name: "Install AltTab preferences",
		Execute: func(i install.Installation) error {
			altTabPlist := filepath.Join(i.PreferencesDir(), "/com.lwouis.alt-tab-macos.plist")
			common.CopyFileFromEmbedFS("alt-tab/Settings.xml", altTabPlist)

			// set up blacklist
			var mappedStrings []string
			for _, app := range i.Blacklist {
				bundle := param.AppToBundleMapping[strings.ToLower(app)]
				mappedStrings = append(mappedStrings, fmt.Sprintf(`{"ignore":"0","bundleIdentifier":"%s","hide":"1"}`, bundle))
			}

			result := "[" + strings.Join(mappedStrings, ",") + "]"

			common.ReplaceWordInFile(altTabPlist, "_BLACKLIST_", result)

			plutilCmd := fmt.Sprintf("plutil -convert binary1 %s", altTabPlist)
			i.Run(plutilCmd)

			i.Run("defaults read com.lwouis.alt-tab-macos.plist")
			return nil
		},
	}
}

func OpenAltTab() Task {
	return Task{
		Name: "Open AtlTab.app",
		Execute: func(i install.Installation) error {
			i.Run("open -a AltTab")
			return nil
		},
	}
}

func ApplySystemSettings() Task {
	return Task{
		Name: "Apply system settings",
		Execute: func(i install.Installation) error {
			optionsMap := make(map[string]bool)
			for _, value := range i.SystemSettings {
				optionsMap[strings.ToLower(value)] = true
			}

			if optionsMap["enable dock auto-hide (2s delay)"] {
				i.Run("defaults write com.apple.dock autohide -bool true")
				i.Run("defaults write com.apple.dock autohide-delay -float 2 && killall Dock")
			}

			if optionsMap[`change dock minimize animation to "scale"`] {
				i.Run(`defaults write com.apple.dock "mineffect" -string "scale" && killall Dock`)
			}

			if optionsMap["enable home & end keys"] {
				common.CopyFileFromEmbedFS("system/DefaultKeyBinding.dict", filepath.Join(i.LibraryDir(), "/KeyBindings/DefaultKeyBinding.dict"))
			}

			if optionsMap["show hidden files in finder"] {
				i.Run("defaults write com.apple.finder AppleShowAllFiles -bool true")
			}

			if optionsMap["show directories on top in finder"] {
				i.Run("defaults write com.apple.finder _FXSortFoldersFirst -bool true")
			}
			if optionsMap["show full posix paths in finder window title"] {
				i.Run("defaults write com.apple.finder _FXShowPosixPathInTitle -bool true")
			}
			return nil
		},
	}
}

func ApplyRules(i install.Installation, file string) {
	common.CopyFileFromEmbedFS(filepath.Join("karabiner", file), filepath.Join(i.KarabinerComplexModificationsDir(), file))
	jq := fmt.Sprintf("jq --arg PROFILE_NAME \"%s\" '(.profiles[] | select(.name == \"%s\") | .complex_modifications.rules) += $rules[].rules' %s --slurpfile rules %s/%s >tmp && mv tmp %s", i.ProfileName, i.ProfileName, i.KarabinerConfigFile(), i.KarabinerComplexModificationsDir(), file, i.KarabinerConfigFile())
	i.Run(jq)
}

func InstallIdeKeymap(i install.Installation, ide param.IDE) error {

	var destDirs []string

	if ide.MultipleDirs {
		destDirs = i.IdeKeymapPaths(ide)
	} else {
		destDirs = []string{filepath.Join(i.Path, ide.ParentDir, ide.Dir, ide.KeymapsDir, ide.DestKeymapsFile)}
	}

	if len(destDirs) == 0 {
		common.PrintColored(common.Yellow, fmt.Sprintf("%s not found. Skipping...", ide.FullName))
		return nil
	}

	for _, d := range destDirs {
		err := common.CopyFileFromEmbedFS(i.SourceKeymap(ide), d)

		if err != nil {
			return err
		}
	}

	return nil
}
