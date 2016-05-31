package xliff

import (
	"crypto/sha1"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/toolani/go-translation-api/trans"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Xliff struct {
	XMLName   xml.Name  `xml:"xliff"`
	Namespace string    `xml:"xmlns,attr"`
	Version   string    `xml:"version,attr"`
	File      XliffFile `xml:"file"`
}

type XliffFile struct {
	Date     string      `xml:"date,attr"`
	DataType string      `xml:"datatype,attr"`
	Original string      `xml:"original,attr"`
	Header   XliffHeader `xml:"header"`
	XliffDomain
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
	Hash             string `xml:"id,attr"`
	TransUnitName    string `xml:"resname,attr"`
	Source           string `xml:"source"`
	TransUnitContent string `xml:"target"`
}

func (xs XliffString) Name() string {
	return xs.TransUnitName
}
func (xs XliffString) Translations() map[trans.Language]trans.Translation {
	ts := make(map[trans.Language]trans.Translation)
	ts[*xs.language] = xs

	return ts
}
func (xs XliffString) Content() string {
	return xs.TransUnitContent
}

func infoFromFilename(filename string) (name string, expectLang string, err error) {
	parts := strings.Split(filename, ".")
	if len(parts) != 3 {
		return "", "", errors.New(fmt.Sprintf("Domain name or language missing from filename '%v'", filename))
	}

	return parts[0], parts[1], nil
}

func hash(input string) (hash string) {
	h := sha1.New()
	h.Write([]byte(input))
	sum := h.Sum(nil)

	return fmt.Sprintf("%x", sum)
}

func New(name, sourceLang, targetLang string) (xliff *Xliff) {
	xliff = &Xliff{Namespace: "urn:oasis:names:tc:xliff:document:1.2", Version: "1.2"}

	xliff.File.Date = "2014-10-15T16:00:00Z"
	xliff.File.Date = time.Now().Format(time.RFC3339)
	xliff.File.DataType = "plaintext"
	xliff.File.Original = "not.available"

	xliff.File.Header.Tool.Id = "go-translation-api"
	xliff.File.Header.Tool.Name = "go-translation-api"
	xliff.File.Header.Tool.Version = "1.0.0-alpha"

	xliff.File.XliffDomain.name = name
	xliff.File.XliffDomain.SourceLang = sourceLang
	xliff.File.XliffDomain.TargetLang = targetLang

	return xliff
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

func getTranslation(s trans.String, l trans.Language) (t trans.Translation) {
	if t, ok := s.Translations()[l]; ok {
		return t
	}

	return nil
}

func Export(source trans.Domain, sourceLang trans.Language, dir string) (err error) {

	// Create output directory
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	xliffs := make(map[trans.Language]*Xliff)

	// Create our set of xliffs
	for _, s := range source.Strings() {
		// The translation's 'source' text, either the content in the target language, or the string
		// name if content is not available
		sourceText := s.Name()
		sourceTrans := getTranslation(s, sourceLang)
		if sourceTrans != nil {
			sourceText = sourceTrans.Content()
		}

		for l, t := range s.Translations() {
			if _, ok := xliffs[l]; !ok {
				xliffs[l] = New(source.Name(), sourceLang.Code, l.Code)
			}
			xliff := xliffs[l]

			xs := &XliffString{
				language:         &trans.Language{Id: l.Id, Code: l.Code, Name: l.Name},
				Hash:             hash(s.Name()),
				TransUnitName:    s.Name(),
				TransUnitContent: t.Content(),
				Source:           sourceText,
			}
			xliff.File.XliffDomain.TransUnits = append(xliff.File.XliffDomain.TransUnits, xs)
			xliffs[l] = xliff
		}
	}

	// Export each xliff to file
	for _, xliff := range xliffs {
		fileName := fmt.Sprintf("%v.%v.xliff", xliff.File.XliffDomain.name, xliff.File.XliffDomain.TargetLang)
		f, err := os.Create(filepath.Join(dir, fileName))
		if err != nil {
			return err
		}

		_, err = f.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
		if err != nil {
			return err
		}
		enc := xml.NewEncoder(f)
		enc.Indent("", "  ")
		if err = enc.Encode(xliff); err != nil {
			return err
		}
		f.Close()
	}

	return nil
}
