package trans

type Domain interface {
	Name() string
	SetName(string)
	Language() string
	Translations() []Translation
}

type Translation struct {
	Id       int
	Language *Language
	Name     string `xml:"resname,attr"`
	Content  string `xml:"target"`
}

type Language struct {
	Id   int
	Code string
	Name string
}
