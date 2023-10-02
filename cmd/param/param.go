package param

import (
	"errors"
	"github.com/AlecAivazis/survey/v2"
	"github.com/raxigan/pcfy-my-mac/cmd/common"
	"gopkg.in/yaml.v3"
	"slices"
)

type Params struct {
	AppLauncher    string
	Terminal       string
	KeyboardLayout string
	Ides           []string
	SystemSettings []string
	Blacklist      []string
}

type FileParams struct {
	AppLauncher    *string `yaml:"app-launcher"`
	Terminal       *string
	KeyboardLayout *string `yaml:"keyboard-layout"`
	Ides           *[]string
	SystemSettings *[]string `yaml:"system-settings"`
	Blacklist      *[]string
	Extra          map[string]string `yaml:",inline"`
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

	validationErr := ValidateAll(
		func() error {
			if fp.AppLauncher != nil {
				return ValidateParamValues("app-launcher", &[]string{*fp.AppLauncher}, []string{Spotlight, Launchpad, Alfred, None})
			}

			return nil
		},
		func() error {
			if fp.Terminal != nil {
				return ValidateParamValues("terminal", &[]string{*fp.Terminal}, []string{Default, ITerm, Warp, None})
			}

			return nil
		},
		func() error {
			if fp.KeyboardLayout != nil {
				return ValidateParamValues("keyboard-layout", &[]string{*fp.KeyboardLayout}, []string{PC, Mac, None})
			}

			return nil
		},
		func() error {
			return ValidateParamValues("ides", fp.Ides, append(IdeKeymapOptions(), []string{"all"}...))
		},
		func() error {
			return ValidateParamValues("system-settings", fp.SystemSettings, SystemSettings)
		},
	)

	if validationErr != nil {
		return FileParams{}, validationErr
	}

	return FileParams{
		AppLauncher:    fp.AppLauncher,
		Terminal:       fp.Terminal,
		KeyboardLayout: fp.KeyboardLayout,
		Ides:           fp.Ides,
		SystemSettings: fp.SystemSettings,
		Blacklist:      fp.Blacklist,
	}, nil
}

func CollectParams(fileParams FileParams) Params {

	questionsToAsk := questions

	fp := Params{}

	m := map[string]bool{
		"appLauncher":    fileParams.AppLauncher != nil,
		"terminal":       fileParams.Terminal != nil,
		"keyboardLayout": fileParams.KeyboardLayout != nil,
		"ides":           fileParams.Ides != nil,
		"blacklist":      fileParams.Blacklist != nil,
		"systemSettings": fileParams.SystemSettings != nil,
	}

	for k, v := range m {
		if v {
			questionsToAsk = slices.DeleteFunc(questionsToAsk, func(e *survey.Question) bool { return e.Name == k })
		}
	}

	common.HandleInterrupt(survey.Ask(questionsToAsk, &fp, survey.WithRemoveSelectAll(), survey.WithRemoveSelectNone(), survey.WithKeepFilter(false)))

	return Params{
		AppLauncher:    common.GetOrDefaultString(fp.AppLauncher, fileParams.AppLauncher),
		Terminal:       common.GetOrDefaultString(fp.Terminal, fileParams.Terminal),
		KeyboardLayout: common.GetOrDefaultString(fp.KeyboardLayout, fileParams.KeyboardLayout),
		Ides:           common.GetOrDefaultSlice(fp.Ides, fileParams.Ides),
		Blacklist:      common.GetOrDefaultSlice(fp.Blacklist, fileParams.Blacklist),
		SystemSettings: common.GetOrDefaultSlice(fp.SystemSettings, fileParams.SystemSettings),
	}
}
