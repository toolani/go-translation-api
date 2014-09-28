package xliff

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/petert82/trans-api/trans"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type Xliff struct {
	XMLName xml.Name  `xml:"xliff"`
	File    XliffFile `xml:"file"`
	Version string    `xml:"version,attr"`
}

type XliffFile struct {
	XliffDomain
	Date     string      `xml:"date,attr"`
	DataType string      `xml:"datatype,attr"`
	Original string      `xml:"original,attr"`
	Header   XliffHeader `xml:"header"`
}

type XliffHeader struct {
	Tool XliffTool `xml:"tool"`
	Note string    `xml:"note"`
}

type XliffTool struct {
	Id      string `xml:"tool-id,attr"`
	Name    string `xml:"tool-name,attr"`
	Version string `xml:"tool-version,attr"`
}

type XliffDomain struct {
	name              string
	SourceLang        string             `xml:"source-language,attr"`
	TargetLang        string             `xml:"target-language,attr"`
	XliffTranslations []XliffTranslation `xml:"body>trans-unit"`
}

func (xd XliffDomain) Name() string {
	return xd.name
}
func (xd *XliffDomain) SetName(name string) {
	xd.name = name
}
func (xd XliffDomain) Language() string {
	return xd.TargetLang
}
func (xd XliffDomain) Translations() []trans.Translation {
	ts := make([]trans.Translation, len(xd.XliffTranslations))
	for i, t := range xd.XliffTranslations {
		ts[i] = t.Translation
	}

	return ts
}

type XliffTranslation struct {
	trans.Translation
	Source string `xml:"source"`
}

func infoFromFilename(filename string) (name string, expectLang string, err error) {
	parts := strings.Split(filename, ".")
	if len(parts) != 3 {
		return "", "", errors.New(fmt.Sprintf("Domain name or language missing from filename '%v'", filename))
	}

	return parts[0], parts[1], nil
}

// Creates a new Xliff from the file at the given path
func NewFromFile(file string) (xliff *Xliff, err error) {
	xliffData, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	xliff = &Xliff{}
	err = xml.Unmarshal(xliffData, xliff)
	if err != nil {
		return nil, err
	}

	if name, expectLang, err := infoFromFilename(filepath.Base(file)); err != nil {
		return nil, err
	} else {
		if xliff.File.XliffDomain.Language() != expectLang {
			return nil, errors.New(fmt.Sprintf(
				"Found language %v but expected %v based on filename '%v' ",
				xliff.File.XliffDomain.Language(),
				expectLang,
				file))
		}

		xliff.File.XliffDomain.SetName(name)

		return xliff, nil
	}
}
