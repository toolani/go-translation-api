package trans

// A whole translation 'domain'
type Domain interface {
	Name() string
	SetName(string)
	Strings() []String
}

// A translatable string
type String interface {
	Name() string
	Translations() map[Language]Translation
}

// A translation of a string
type Translation interface {
	Content() string
}

type Language struct {
	Id   int64
	Code string // language / locale code
	Name string
}
