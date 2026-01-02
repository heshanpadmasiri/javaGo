package converted

type SimpleClass struct {
	value int
	name  string
}

func NewSimpleClass() SimpleClass {
	this := SimpleClass{}
	return this
}
