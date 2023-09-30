package install

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/bitfield/script"
	"github.com/raxigan/pcfy-my-mac/configs"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func copyFileFromEmbedFS(src, dst string) error {
	configs := &configs.Configs
	data, _ := fs.ReadFile(configs, src)
	os.MkdirAll(filepath.Dir(dst), 0755)
	return os.WriteFile(dst, data, 0755)
}

type Params struct {
	AppLauncher       string
	Terminal          string
	KeyboardLayout    string
	Ides              []IDE
	AdditionalOptions []string
	Blacklist         []string
}

type Installation struct {
	Commander
	profileName      string
	installationTime time.Time
}

type FileParams struct {
	AppLauncher       *string `yaml:"app-launcher"`
	Terminal          *string
	KeyboardLayout    *string `yaml:"keyboard-layout"`
	Ides              *[]string
	AdditionalOptions *[]string `yaml:"additional-options"`
	Blacklist         *[]string
	Extra             map[string]string `yaml:",inline"`
}

func CollectYamlParams(yml string) (FileParams, error) {

	fp := FileParams{}

	err := yaml.Unmarshal([]byte(yml), &fp)
	if err != nil {
		return FileParams{}, err
	}

	if len(fp.Extra) > 0 {
		for field := range fp.Extra {
			return FileParams{}, errors.New("Unknown parameter: " + field)
		}
	}

	validationErr := validateAll(
		func() error {
			if fp.AppLauncher != nil {
				return validateParamValues("app-launcher", &[]string{*fp.AppLauncher}, []string{Spotlight.String(), Launchpad.String(), Alfred.String(), "None"})
			}

			return nil
		},
		func() error {
			if fp.Terminal != nil {
				return validateParamValues("terminal", &[]string{*fp.Terminal}, []string{Default.String(), iTerm.String(), Warp.String(), "None"})
			}

			return nil
		},
		func() error {
			if fp.KeyboardLayout != nil {
				return validateParamValues("keyboard-layout", &[]string{*fp.KeyboardLayout}, []string{PC.String(), Mac.String(), "None"})
			}

			return nil
		},
		func() error {
			return validateParamValues("ides", fp.Ides, append(IdeKeymapOptions(), []string{"all"}...))
		},
		func() error {
			return validateParamValues("additional-options", fp.AdditionalOptions, AdditionalOptions)
		},
	)

	if validationErr != nil {
		return FileParams{}, validationErr
	}

	return FileParams{
		AppLauncher:       fp.AppLauncher,
		Terminal:          fp.Terminal,
		KeyboardLayout:    fp.KeyboardLayout,
		Ides:              fp.Ides,
		AdditionalOptions: fp.AdditionalOptions,
		Blacklist:         fp.Blacklist,
	}, nil
}

func makeSurvey(s MySurvey) string {

	appLauncher := ""

	prompt := &survey.Select{
		Message: s.Message,
		Options: append(s.Options, "None"),
		Default: s.Options[0],
	}

	appLauncher = strings.TrimSpace(appLauncher)

	handleInterrupt(survey.AskOne(prompt, &appLauncher, survey.WithValidator(survey.Required)))

	return strings.ToLower(strings.TrimSpace(appLauncher))
}

func makeMultiSelect(s survey.MultiSelect) []string {
	var appLauncher []string
	handleInterrupt(survey.AskOne(&s, &appLauncher))
	return appLauncher
}

func RunInstaller(homeDir HomeDir, commander Commander, tp TimeProvider, params Params) error {

	installation := Installation{
		Commander:        commander,
		profileName:      "PC mode GOLANG",
		installationTime: tp.Now(),
	}

	return installation.install(params, homeDir)
}

func CollectParams(params FileParams) Params {

	var app string
	var term string
	var kbType string
	var idesToInstall []IDE
	var blacklist []string

	appLauncherSurvey := MySurvey{
		Message: "App Launcher (will be available with Win(⊞)/Opt(⌥) key):",
		Options: []string{Spotlight.String(), Launchpad.String(), Alfred.String()},
	}

	terminalSurvey := MySurvey{
		Message: "What is your terminal of choice (will be available with Ctrl+Alt+T/Ctrl+Cmd+T shortcut):",
		Options: []string{Default.String(), iTerm.String(), Warp.String()},
	}

	kbTypeSurvey := MySurvey{
		Message: "Your external keyboard layout:",
		Options: []string{PC.String(), Mac.String()},
	}

	if params.AppLauncher == nil {
		app = makeSurvey(appLauncherSurvey)
	} else {
		app = *params.AppLauncher
	}

	if params.Terminal == nil {
		term = makeSurvey(terminalSurvey)
	} else {
		term = *params.Terminal
	}

	if params.KeyboardLayout == nil {
		kbType = makeSurvey(kbTypeSurvey)
	} else {
		kbType = *params.KeyboardLayout
	}

	if params.Ides == nil {

		ideSurvey := survey.MultiSelect{
			Message: "IDE keymaps to install:",
			Options: IdeKeymapsSurveyOptions(),
			Help:    "help",
		}

		fullNames := makeMultiSelect(ideSurvey)

		for _, e := range fullNames {
			name, _ := IdeKeymapByFullName(e)
			idesToInstall = append(idesToInstall, name)
		}
	} else {

		if slices.Contains(*params.Ides, "all") {
			idesToInstall = IDEKeymaps
		} else {

			var idesFromFlags []IDE

			for _, e := range *params.Ides {
				if e != "" {
					byFlag, _ := IdeKeymapByFullName(e)
					idesFromFlags = append(idesFromFlags, byFlag)
				}
			}

			idesToInstall = idesFromFlags
		}
	}

	if params.Blacklist == nil {

		msBlacklist := survey.MultiSelect{
			Message: "Select apps to be blacklisted:",
			Options: []string{
				"Spotify",
				"Finder",
				"System Preferences",
			},
			Help: "help",
		}

		blacklist = makeMultiSelect(msBlacklist)
	} else {
		blacklist = *params.Blacklist
	}

	var options []string

	if params.AdditionalOptions == nil {

		ms := survey.MultiSelect{
			Message: "Select additional options:",
			Options: []string{
				"Enable Dock auto-hide (2s delay)",
				`Change Dock minimize animation to "scale"`,
				"Enable Home & End keys",
				"Show hidden files in Finder",
				"Show directories on top in Finder",
				"Show full POSIX paths in Finder",
			},
			Description: func(value string, index int) string {
				if index < 2 {
					return "Recommended"
				}
				return ""
			},
			Help:     "help",
			PageSize: 15,
		}

		options = makeMultiSelect(ms)
	} else {
		options = *params.AdditionalOptions
	}

	return Params{
		AppLauncher:       app,
		Terminal:          term,
		KeyboardLayout:    kbType,
		Ides:              idesToInstall,
		AdditionalOptions: options,
		Blacklist:         blacklist,
	}
}

func (i Installation) install(params Params, home HomeDir) error {

	i.tryInstallDependencies()

	i.Run("killall Karabiner-Elements")

	// do karabiner.json backup
	original := home.KarabinerConfigFile()
	backupDest := home.KarabinerConfigBackupFile(i.installationTime)

	script.Exec("cp " + original + " " + backupDest).Wait()

	// delete existing profile
	deleteProfileJqCmd := fmt.Sprintf("jq --arg PROFILE_NAME \"%s\" 'del(.profiles[] | select(.name == \"%s\"))' %s >tmp && mv tmp %s", i.profileName, i.profileName, home.KarabinerConfigFile(), home.KarabinerConfigFile())
	i.Run(deleteProfileJqCmd)

	// add new karabiner profile
	copyFileFromEmbedFS("karabiner/karabiner-profile.json", "tmp")
	addProfileJqCmd := fmt.Sprintf("jq '.profiles += $profile' %s --slurpfile profile tmp --indent 4 >INPUT.tmp && mv INPUT.tmp %s && rm tmp", home.KarabinerConfigFile(), home.KarabinerConfigFile())
	i.Run(addProfileJqCmd)

	// rename the profile
	renameJqCmd := fmt.Sprintf("jq '.profiles |= map(if .name == \"_PROFILE_NAME_\" then .name = \"%s\" else . end)' %s > tmp && mv tmp %s", i.profileName, home.KarabinerConfigFile(), home.KarabinerConfigFile())
	i.Run(renameJqCmd)

	// unselect other profiles
	unselectJqCmd := fmt.Sprintf("jq '.profiles |= map(if .name != \"%s\" then .selected = false else . end)' %s > tmp && mv tmp %s", i.profileName, home.KarabinerConfigFile(), home.KarabinerConfigFile())
	i.Run(unselectJqCmd)

	i.applyRules(home, "main.json")
	i.applyRules(home, "finder.json")

	switch strings.ToLower(params.AppLauncher) {
	case "spotlight":
		i.applyRules(home, "spotlight.json")
	case "launchpad":
		i.applyRules(home, "launchpad.json")
	case "alfred":
		{
			if i.Exists("Alfred 4.app") || i.Exists("Alfred 5.app") {

				i.applyRules(home, "alfred.json")

				dirs, err := findMatchingDirs(home.ApplicationSupportDir()+"/Alfred/Alfred.alfredpreferences/preferences/local", "", "hotkey", "prefs.plist")

				if err != nil {
					return err
				}

				for _, e := range dirs {
					copyFileFromEmbedFS("alfred/prefs.plist", e)
				}
			} else {
				printColored(YELLOW, fmt.Sprintf("Alfred app not found. Skipping..."))
			}
		}
	}

	switch strings.ToLower(params.KeyboardLayout) {
	case "mac":
		prepareForExternalMacKeyboard(home, i)
	}

	switch strings.ToLower(params.Terminal) {
	case "default":
		i.applyRules(home, "apple-terminal.json")
	case "iterm":
		if i.Exists("iTerm.app") {
			i.applyRules(home, "iterm.json")
		} else {
			printColored(YELLOW, fmt.Sprintf("iTerm app not found. Skipping..."))
		}
	case "warp":
		{
			if i.Exists("Warp.app") {
				i.applyRules(home, "warp.json")
			} else {
				printColored(YELLOW, fmt.Sprintf("Warp app not found. Skipping..."))
			}
		}
	}

	// reformat using 2 spaces indentation
	i.Run(fmt.Sprintf("jq '.' %s > tmp && mv tmp %s", home.KarabinerConfigFile(), home.KarabinerConfigFile()))

	i.Run("open -a Karabiner-Elements")

	for _, ide := range params.Ides {
		i.installIdeKeymap(home, ide)
	}

	i.Run("killall Rectangle")

	rectanglePlist := home.PreferencesDir() + "/com.knollsoft.Rectangle.plist"
	copyFileFromEmbedFS("rectangle/Settings.xml", rectanglePlist)

	plutilCmdRectangle := fmt.Sprintf("plutil -convert binary1 %s", rectanglePlist)
	i.Run(plutilCmdRectangle)
	i.Run("defaults read com.knollsoft.Rectangle.plist")
	i.Run("open -a Rectangle")

	i.Run("killall AltTab")

	altTabPlist := home.PreferencesDir() + "/com.lwouis.alt-tab-macos.plist"
	copyFileFromEmbedFS("alt-tab/Settings.xml", altTabPlist)

	// set up blacklist

	var mappedStrings []string
	for _, s := range params.Blacklist {
		mappedStrings = append(mappedStrings, fmt.Sprintf(`{"ignore":"0","bundleIdentifier":"%s","hide":"1"}`, s))
	}

	result := "[" + strings.Join(mappedStrings, ",") + "]"

	replaceWordInFile(altTabPlist, "_BLACKLIST_", result)

	plutilCmd := fmt.Sprintf("plutil -convert binary1 %s", altTabPlist)
	i.Run(plutilCmd)

	i.Run("defaults read com.lwouis.alt-tab-macos.plist")
	i.Run("open -a AltTab")

	optionsMap := make(map[string]bool)
	for _, value := range params.AdditionalOptions {
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
		copyFileFromEmbedFS("system/DefaultKeyBinding.dict", home.LibraryDir()+"/KeyBindings/DefaultKeyBinding.dict")
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

	fmt.Println("SUCCESS")

	return nil
}

func (i Installation) tryInstallDependencies() {

	var notInstalled []string
	var commands []string

	if !i.Exists("jq") {
		notInstalled = append(notInstalled, "jq")
		commands = append(commands, "brew install jq")
	}

	if !i.Exists("Karabiner-Elements.app") {
		notInstalled = append(notInstalled, "Karabiner-Elements")
		commands = append(commands, "brew install --cask karabiner-elements")
	}

	if !i.Exists("AltTab.app") {
		notInstalled = append(notInstalled, "AltTab")
		commands = append(commands, "brew install --cask alt-tab")
	}

	if !i.Exists("Rectangle.app") {
		notInstalled = append(notInstalled, "Rectangle")
		commands = append(commands, "brew install --cask rectangle")
	}

	if len(notInstalled) > 0 {
		installApp := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("The following dependencies will be installed: %s. Do you agree?", strings.Join(notInstalled, ", ")),
		}
		handleInterrupt(survey.AskOne(prompt, &installApp))

		if !installApp {
			fmt.Printf("Qutting...")
			i.Exit(0)
		}

		for _, c := range commands {
			i.Run(c)
		}
	}
}

func (i Installation) installIdeKeymap(home HomeDir, ide IDE) error {

	var destDirs []string

	if ide.multipleDirs {
		a := home.IdeKeymapPaths(ide)

		destDirs = a
	} else {
		destDirs = []string{
			home.Path + "/" + ide.parentDir + "/" + ide.dir + "/" + ide.keymapsDir + "/" + ide.destKeymapsFile,
		}
	}

	if len(destDirs) == 0 {
		printColored(YELLOW, fmt.Sprintf("%s not found. Skipping...", ide.fullName))
		return nil
	}

	for _, d := range destDirs {
		err := copyFileFromEmbedFS(home.SourceKeymap(ide), d)

		if err != nil {
			return err
		}
	}

	return nil
}

func findMatchingDirs(basePath, namePrefix, subDir, fileName string) ([]string, error) {

	var result []string

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {

		if path != basePath && strings.HasPrefix(info.Name(), namePrefix) {
			if err != nil {
				return err
			}

			if fileExists(filepath.Join(basePath, info.Name())) {
				destDir := filepath.Join(path, subDir)
				destFilePath := filepath.Join(destDir, fileName)
				result = append(result, destFilePath)
			}
		}

		return nil
	})

	return result, err
}

func (home HomeDir) IdeKeymapPaths(ide IDE) []string {
	return home.IdesKeymapPaths([]IDE{ide})
}

func (home HomeDir) IdesKeymapPaths(ide []IDE) []string {

	var result []string

	for _, e := range ide {

		dirs, _ := findMatchingDirs(home.Path+e.parentDir, e.dir, e.keymapsDir, e.destKeymapsFile)

		for _, e1 := range dirs {
			result = append(result, e1)
		}
	}

	return result
}

func (i Installation) applyRules(home HomeDir, file string) {
	copyFileFromEmbedFS("karabiner/"+file, home.KarabinerComplexModificationsDir()+"/"+file)
	jq := fmt.Sprintf("jq --arg PROFILE_NAME \"%s\" '(.profiles[] | select(.name == \"%s\") | .complex_modifications.rules) += $rules[].rules' %s --slurpfile rules %s/%s >tmp && mv tmp %s", i.profileName, i.profileName, home.KarabinerConfigFile(), home.KarabinerComplexModificationsDir(), file, home.KarabinerConfigFile())
	i.Run(jq)
}

func prepareForExternalMacKeyboard(home HomeDir, i Installation) {
	jq := fmt.Sprintf("jq --arg PROFILE_NAME \"%s\" '.profiles |= map(if .name == \"%s\" then walk(if type == \"object\" and .conditions then del(.conditions[] | select(.identifiers[]?.is_built_in_keyboard)) else . end) else . end)' %s --indent 4 >tmp && mv tmp %s", i.profileName, i.profileName, home.KarabinerConfigFile(), home.KarabinerConfigFile())
	i.Run(jq)
}

func validateParamValues(param string, values *[]string, validValues []string) error {

	if values != nil && len(*values) != 0 {

		vals := toLowerSlice(*values)
		valids := toLowerSlice(validValues)

		validMap := make(map[string]bool)
		for _, v := range valids {
			validMap[v] = true
		}

		var invalidValues []string
		for _, val := range vals {
			if !validMap[val] {
				invalidValues = append(invalidValues, val)
			}
		}

		if len(invalidValues) != 0 {
			joined := strings.Join(invalidValues, ", ")
			return errors.New("Invalid param '" + param + "' value/s '" + joined + "', valid values:\n" + strings.Join(validValues, "\n"))
		}
	}

	return nil
}

func toLowerSlice(slice []string) []string {
	for i, s := range slice {
		slice[i] = strings.ToLower(s)
	}
	return slice
}

func validateAll(params ...func() error) error {
	for _, paramFunc := range params {
		if err := paramFunc(); err != nil {
			return err
		}
	}
	return nil
}

func handleInterrupt(err error) {
	if errors.Is(err, terminal.InterruptErr) {
		fmt.Println("Quitting...")
		os.Exit(1)
	}
}
