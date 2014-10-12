package xliff

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/petert82/go-translation-api/trans"
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
	name       string
	SourceLang string         `xml:"source-language,attr"`
	TargetLang string         `xml:"target-language,attr"`
	TransUnits []*XliffString `xml:"body>trans-unit"`
}

func (xd XliffDomain) Name() string {
	return xd.name
}
func (xd *XliffDomain) SetName(name string) {
	xd.name = name
}
func (xd XliffDomain) Strings() []trans.String {
	ss := make([]trans.String, len(xd.TransUnits))
	for i, s := range xd.TransUnits {
		ss[i] = s
	}

	return ss
}

type XliffString struct {
	language         *trans.Language
	TransUnitName    string `xml:"resname,attr"`
	TransUnitContent string `xml:"target"`
	Source           string `xml:"source"`
}

func (xs *XliffString) Name() string {
	return xs.TransUnitName
}
func (xs *XliffString) SetName(name string) {
	xs.TransUnitName = name
}
func (xs *XliffString) Translations() map[trans.Language]trans.Translation {
	ts := make(map[trans.Language]trans.Translation)
	ts[*xs.language] = xs

	return ts
}
func (xs *XliffString) Content() string {
	return xs.TransUnitContent
}
func (xs *XliffString) SetContent(content string) {
	xs.TransUnitContent = content
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
		if xliff.File.XliffDomain.TargetLang != expectLang {
			return nil, errors.New(fmt.Sprintf(
				"Found language '%v' but expected '%v' based on filename '%v' ",
				xliff.File.XliffDomain.TargetLang,
				expectLang,
				file))
		}

		xliff.File.XliffDomain.SetName(name)

		l := trans.Language{Code: xliff.File.XliffDomain.TargetLang}
		for _, s := range xliff.File.XliffDomain.TransUnits {
			s.language = &l
		}

		return xliff, nil
	}
}
