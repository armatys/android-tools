package crowdin

import (
	"archive/zip"
	"fmt"
	"github.com/daaku/go.httpzip"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
)

type CrowdinConfig struct {
	Key          string
	ProjectName  string
	FileName     string
	LocaleToCopy []string
}

var validLocaleRegexp *regexp.Regexp = regexp.MustCompile("^[a-z]{2}(\\-[A-Z]{2})?/")
var hyphenRegexp *regexp.Regexp = regexp.MustCompile("-")

func ExportStrings(config *CrowdinConfig) (string, error) {
	url := fmt.Sprintf("http://api.crowdin.net/api/project/%s/export?key=%s", config.ProjectName, config.Key)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var buf []byte
	_, err = resp.Body.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func shouldCopyTranslations(config *CrowdinConfig, localeIdentifier string) bool {
	if len(config.LocaleToCopy) == 0 {
		return true
	}
	for _, l := range config.LocaleToCopy {
		if l == localeIdentifier {
			return true
		}
	}
	return false
}

func UpdateStrings(config *CrowdinConfig, resDir, stringsFilename string) error {
	expr := fmt.Sprintf("^([a-zA-Z\\-]+)/%s\\.xml", config.FileName)
	stringsFileRegex, err := regexp.Compile(expr)
	if err != nil {
		return err
	}

	log.Println("Downloading zip file")
	url := fmt.Sprintf("http://api.crowdin.net/api/project/%s/download/all.zip?key=%s", config.ProjectName, config.Key)
	zipReader, err := httpzip.ReadURL(url)
	if err != nil {
		return err
	}

	log.Printf("Extracting into %s directory...", resDir)
	for _, f := range zipReader.File {
		if match := stringsFileRegex.FindStringSubmatch(f.FileHeader.Name); match != nil && validLocaleRegexp.MatchString(f.FileHeader.Name) {
			localeIdentifier := match[1]
			if shouldCopyTranslations(config, localeIdentifier) {
				if err := copyStringsToResources(f, localeIdentifier, stringsFilename, resDir); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func copyStringsToResources(f *zip.File, localeIdentifier, stringsFilename, resDir string) error {
	valuesDirName := fmt.Sprintf("values-%s", hyphenRegexp.ReplaceAllLiteralString(localeIdentifier, "-r"))
	targetValuesDir := path.Join(resDir, valuesDirName)
	targetStringsFilename := path.Join(targetValuesDir, stringsFilename)

	if err := os.MkdirAll(targetValuesDir, 0755); err != nil {
		return err
	}

	log.Printf("Copying %s to %s\n", f.FileHeader.Name, targetStringsFilename)

	sourceFile, err := f.Open()
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetStringsFilename)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}

	return nil
}
