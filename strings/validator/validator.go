package validator

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
)

type stringEl struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type pluralItemEl struct {
	Quantity string `xml:"quantity,attr"`
	Value    string `xml:",chardata"`
}

type pluralEl struct {
	Name  string         `xml:"name,attr"`
	Items []pluralItemEl `xml:"item"`
}

type stringArrayEl struct {
	Name  string   `xml:"name,attr"`
	Items []string `xml:"item"`
}

type resourcesEl struct {
	Strings      []stringEl      `xml:"string"`
	Plurals      []pluralEl      `xml:"plurals"`
	StringArrays []stringArrayEl `xml:"string-array"`
}

type ResourceMissingError struct {
	msg string
}

func (r *ResourceMissingError) Error() string {
	return r.msg
}

type ValidationError struct {
	msg string
}

func (v *ValidationError) Error() string {
	return v.msg
}

// A type of function that validates the `validatedString` based on the `baseString`.
type comparisonValidation func(baseString, validatedString string) error

// A type of function that validates if `s` is valid.
type simpleValidation func(s string) error

var SimplePlaceholderRegex *regexp.Regexp = regexp.MustCompile("(\\%[a-zA-Z])")
var PositionalPlaceholderRegex *regexp.Regexp = regexp.MustCompile("(\\%[0-9]+\\$[a-zA-Z])")
var PotentialPlaceholderRegex *regexp.Regexp = regexp.MustCompile("(\\%\\s)")
var NewLineRegex *regexp.Regexp = regexp.MustCompile("(\n)")

// Validate the string resources that are inside the "resDir" directory.
// The XML string file for the "baseLocale" is not validated, but used for comparison.
// Returns a list of errors.
func Validate(resDir, baseLocale, stringsFilename string, showMissing bool) (errorList []error) {
	errorList = make([]error, 0)
	baseResources, err := parseResources(resDir, baseLocale, stringsFilename)
	if err != nil {
		errorList = append(errorList, err)
		return
	}

	paths, err := getOtherStringsFilePaths(resDir, baseLocale, stringsFilename)
	if err != nil {
		errorList = append(errorList, err)
		return
	}

	for _, path := range paths {
		resources, err := parseResourcesFile(path)
		if err != nil {
			errorList = append(errorList, err)
			continue
		}

		shortPath := extractShortPath(resDir, path)
		ers := validateResources(baseResources, resources, shortPath, showMissing)
		errorList = append(errorList, ers...)
	}

	return
}

func valuesDir(locale string) string {
	if len(locale) > 0 {
		return fmt.Sprintf("values-%s", locale)
	}
	return "values"
}

// Constructs the file path from `resDir`, `localeName` and `stringsFilename`,
// and returns parsed resources or an error.
func parseResources(resDir, localeName, stringsFilename string) (*resourcesEl, error) {
	var path string = filepath.Join(resDir, valuesDir(localeName), stringsFilename)
	return parseResourcesFile(path)
}

// Reads a file at a `path` and returns parsed resources object, or an error.
func parseResourcesFile(path string) (*resourcesEl, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var resources resourcesEl
	err = xml.Unmarshal([]byte(data), &resources)
	if err != nil {
		return nil, err
	}
	return &resources, nil
}

// Generates the file paths for other string resource files.
// `resDir` is the path to the Android's "res" directory.
// `exceptForLocale` is the locale of the file path, that will not be included in the returned paths.
// `stringsFilename` is the name of the XML file that contains the string resources (e.g. "strings.xml").
func getOtherStringsFilePaths(resDir, exceptForLocale, stringsFilename string) ([]string, error) {
	patt := filepath.Join(resDir, "values-*", stringsFilename)
	paths, err := filepath.Glob(patt)
	if err != nil {
		return nil, err
	}
	patt2 := filepath.Join(resDir, "values", stringsFilename)
	paths2, err := filepath.Glob(patt2)
	if err != nil {
		return nil, err
	}
	paths = append(paths, paths2...)
	exceptForPath := filepath.Join(resDir, valuesDir(exceptForLocale), stringsFilename)

	idx := -1
	for i, p := range paths {
		if p == exceptForPath {
			idx = i
			break
		}
	}

	if idx >= 0 {
		paths = append(paths[:idx], paths[idx+1:]...)
	}

	return paths, nil
}

func findStringElement(resources *resourcesEl, name string) *stringEl {
	for _, el := range resources.Strings {
		if el.Name == name {
			return &el
		}
	}
	return nil
}

func findStringArrayElement(resources *resourcesEl, name string) *stringArrayEl {
	for _, el := range resources.StringArrays {
		if el.Name == name {
			return &el
		}
	}
	return nil
}

// Extracts the short path for a string file (e.g. "values-en/strings.xml")
// based on the `resDir` path and the `stringsFilePath`.
// If extraction fails, it returns `stringsFilePath`.
func extractShortPath(resDir, stringsFilePath string) string {
	p, err := filepath.Rel(resDir, stringsFilePath)
	if err != nil {
		p = stringsFilePath
	}
	return p
}

// Validates the resources against the `baseResources`, which are expected to contain no errors.
// Returns a list of validation errors.
// If `showMissing` is true, this function returns an error
// when a resource exists in the `baseResources`, but not in `validatedResources`.
func validateResources(baseResources, validatedResources *resourcesEl, shortPath string, showMissing bool) []error {
	var errorList []error

	for _, validatedElem := range validatedResources.Strings {
		hasBaseValue := false
		for _, baseElem := range baseResources.Strings {
			if validatedElem.Name == baseElem.Name {
				hasBaseValue = true
				break
			}
		}
		if !hasBaseValue {
			valError := ValidationError{fmt.Sprintf("%s in %s does not have a base value.", validatedElem.Name, shortPath)}
			errorList = append(errorList, &valError)
		}
	}

	comparisonValidationFuncs := []comparisonValidation{validateSimplePlaceholders, validatePositionalPlaceholders}
	simpleValidationFuncs := []simpleValidation{validatePotentialPlaceholder, validateNewlineCharacters}

	// Validate string elements
	for _, baseElem := range baseResources.Strings {
		validatedElem := findStringElement(validatedResources, baseElem.Name)
		if validatedElem == nil {
			if showMissing {
				errorList = append(errorList, &ResourceMissingError{fmt.Sprintf("[missing] element named %s in %s", baseElem.Name, shortPath)})
			}
			continue
		}
		for _, fn := range comparisonValidationFuncs {
			if err := fn(baseElem.Value, validatedElem.Value); err != nil {
				valError := ValidationError{fmt.Sprintf("%s in %s: %s", baseElem.Name, shortPath, err.Error())}
				errorList = append(errorList, &valError)
			}
		}
		for _, fn := range simpleValidationFuncs {
			if err := fn(validatedElem.Value); err != nil {
				valError := ValidationError{fmt.Sprintf("%s in %s: %s", baseElem.Name, shortPath, err.Error())}
				errorList = append(errorList, &valError)
			}
		}
	}

	// Validate string-array elements
	for _, baseElem := range baseResources.StringArrays {
		validatedElem := findStringArrayElement(validatedResources, baseElem.Name)
		if validatedElem == nil {
			if showMissing {
				errorList = append(errorList, &ResourceMissingError{fmt.Sprintf("[missing] element named %s in %s", baseElem.Name, shortPath)})
			}
			continue
		}
		if len(baseElem.Items) != len(validatedElem.Items) {
			errorList = append(errorList, &ValidationError{fmt.Sprintf("%s array in %s has %d items, but it should have %d", validatedElem.Name, shortPath, len(validatedElem.Items), len(baseElem.Items))})
			continue
		}
		for i := range baseElem.Items {
			for _, fn := range comparisonValidationFuncs {
				if err := fn(baseElem.Items[i], validatedElem.Items[i]); err != nil {
					valError := ValidationError{fmt.Sprintf("%s in %s: %s", baseElem.Name, shortPath, err.Error())}
					errorList = append(errorList, &valError)
				}
			}
			for _, fn := range simpleValidationFuncs {
				if err := fn(validatedElem.Items[i]); err != nil {
					valError := ValidationError{fmt.Sprintf("%s in %s: %s", baseElem.Name, shortPath, err.Error())}
					errorList = append(errorList, &valError)
				}
			}
		}
	}

	// Validate plurals elements
	for _, pluralsElem := range validatedResources.Plurals {
		for _, pluralValue := range pluralsElem.Items {
			for _, fn := range simpleValidationFuncs {
				if err := fn(pluralValue.Value); err != nil {
					valError := ValidationError{fmt.Sprintf("%s in %s: %s", pluralsElem.Name, shortPath, err.Error())}
					errorList = append(errorList, &valError)
				}
			}
		}
	}

	return errorList
}

func validateSimplePlaceholders(baseElemString, validatedElemString string) error {
	baseMatches := SimplePlaceholderRegex.FindAllStringSubmatch(baseElemString, -1)
	targetMatches := SimplePlaceholderRegex.FindAllStringSubmatch(validatedElemString, -1)
	baseMatchesCount := len(baseMatches)
	targetMatchesCount := len(targetMatches)
	if baseMatchesCount == 0 && targetMatchesCount == 0 {
		return nil
	}
	if baseMatchesCount != targetMatchesCount {
		return errors.New(fmt.Sprintf("The target string has %d placeholder(s), while it should probably have %d", targetMatchesCount, baseMatchesCount))
	}
	for i, match := range baseMatches {
		targetMatch := targetMatches[i]
		if match[1] != targetMatch[1] {
			return errors.New(fmt.Sprintf("The target string placeholder #%d is %s, while it probably should be %s", i, targetMatch[1], match[1]))
		}
	}
	return nil
}

func validatePositionalPlaceholders(baseElemString, validatedElemString string) error {
	baseMatches := PositionalPlaceholderRegex.FindAllStringSubmatch(baseElemString, -1)
	targetMatches := PositionalPlaceholderRegex.FindAllStringSubmatch(validatedElemString, -1)
	baseMatchesCount := len(baseMatches)
	targetMatchesCount := len(targetMatches)
	if baseMatchesCount == 0 && targetMatchesCount == 0 {
		return nil
	}
	if baseMatchesCount != targetMatchesCount {
		return errors.New(fmt.Sprintf("The target string has %d placeholder(s), while it should probably have %d", targetMatchesCount, baseMatchesCount))
	}
	for i, match := range baseMatches {
		var foundMatch []string = nil
		for _, tmatch := range targetMatches {
			if match[1] == tmatch[1] {
				foundMatch = tmatch
			}
		}
		if foundMatch == nil {
			return errors.New(fmt.Sprintf("The target string placeholder #%d is %s, while it probably should be %s", i, foundMatch[1], match[1]))
		}
	}
	return nil
}

func validatePotentialPlaceholder(elemValue string) error {
	matches := PotentialPlaceholderRegex.FindAllStringSubmatch(elemValue, -1)
	if len(matches) > 0 {
		return errors.New(fmt.Sprintf("Value '%s' has a potential placeholder", NewLineRegex.ReplaceAllString(elemValue, "\\n")))
	}
	return nil
}

func validateNewlineCharacters(elemValue string) error {
	matches := NewLineRegex.FindAllStringSubmatch(elemValue, -1)
	if len(matches) > 0 {
		return errors.New(fmt.Sprintf("The following line must not have a newline character: '%s'", NewLineRegex.ReplaceAllString(elemValue, "\\n")))
	}
	return nil
}
