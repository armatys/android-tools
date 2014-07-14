package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/armatys/android-tools/strings/crowdin"
	"github.com/armatys/android-tools/strings/validator"
	"os"
)

// The action name to perform.
var actionNameArg string

// The path to the Android's "res" directory.
var projectResDirArg string

// The base locale used for comparison and validation of other locale strings.
var baseLocaleArg string

// The name of the xml file with string resources.
var stringsFileNameArg string

// Flag that specifies it the string validator should show strings that exist in base resources, but not in other resources.
var showMissingArg bool

// Path to a file with configuration for accessing crowdin.
// The file should contain a JSON object like this: {"Key": "api_key", "ProjectName": "the-project-name"}
var crowdinConfigFileArg string

var (
	actionNameValidate      = "validate"
	actionNameCrowdinUpdate = "crowdin-update"
	actionNameCrowdinExport = "crowdin-export"
	supportedActionNames    = []string{actionNameValidate, actionNameCrowdinUpdate, actionNameCrowdinExport}
)

func init() {
	flag.StringVar(&actionNameArg, "action", actionNameValidate, fmt.Sprintf("Action to perform, one of %v.", supportedActionNames))
	flag.StringVar(&projectResDirArg, "resdir", "", "The path to the 'res' directory of your Android project (required for 'validate' and 'crowdin-update').")
	flag.StringVar(&baseLocaleArg, "baselocale", "", "The base locale used for validation of other locale strings (e.g. 'en' or 'en-rGB').")
	flag.StringVar(&stringsFileNameArg, "filename", "strings.xml", "The name of the xml file with XML string resources (required for 'validate' and 'crowdin-update').")
	flag.BoolVar(&showMissingArg, "missing", false, "If true shows the missing translations (use with 'validate').")
	flag.StringVar(&crowdinConfigFileArg, "crowdin-conf", "", "The path to a file with a JSON configuration for accessing Crowdin service (required for 'crowdin-*'). The JSON should look like {\"Key\": \"api_key\", \"ProjectName\": \"the-project-name\"}")
}

func main() {
	flag.Parse()
	if !isActionSupported(actionNameArg) {
		fmt.Printf("Action '%s' is not supported.\n", actionNameArg)
		os.Exit(-1)
	}
	if actionNameArg == actionNameValidate {
		validateStrings()
	} else if actionNameArg == actionNameCrowdinUpdate {
		crowdinUpdate()
	} else if actionNameArg == actionNameCrowdinExport {
		crowdinExport()
	}
}

func validateStrings() {
	if !(len(projectResDirArg) > 0 && len(stringsFileNameArg) > 0) {
		flag.Usage()
		os.Exit(-1)
	}

	var errorList []error = validator.Validate(projectResDirArg, baseLocaleArg, stringsFileNameArg, showMissingArg)
	errorCount := 0

	if len(errorList) > 0 {
		for _, e := range errorList {
			errorCount += 1
			fmt.Printf("[%d] %s\n", errorCount, e.Error())
		}
	}

	if errorCount > 0 {
		fmt.Printf("Found %d errors.\n", errorCount)
	} else {
		fmt.Println("No errors found.")
	}
	os.Exit(errorCount)
}

func crowdinUpdate() {
	if !(len(projectResDirArg) > 0 && len(stringsFileNameArg) > 0) {
		flag.Usage()
		os.Exit(-1)
	}
	config, err := loadCrowdinConf()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}
	if err := crowdin.UpdateStrings(config, projectResDirArg, stringsFileNameArg); err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	} else {
		fmt.Println("Strings have been updated.")
		os.Exit(0)
	}
}

func crowdinExport() {
	config, err := loadCrowdinConf()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	}
	if resp, err := crowdin.ExportStrings(config); err != nil {
		fmt.Println(err.Error())
		os.Exit(-1)
	} else {
		fmt.Println(resp)
		os.Exit(0)
	}
}

func loadCrowdinConf() (*crowdin.CrowdinConfig, error) {
	if len(crowdinConfigFileArg) == 0 {
		return nil, errors.New("The path to Crowdin configuration file is required.")
	}
	file, err := os.Open(crowdinConfigFileArg)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var config crowdin.CrowdinConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// Returns true if the `actionName` is supported by this tool.
func isActionSupported(actionName string) bool {
	for _, name := range supportedActionNames {
		if name == actionName {
			return true
		}
	}
	return false
}
