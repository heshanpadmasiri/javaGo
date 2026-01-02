package converted

type Person struct {
	Name   string
	Age    int
	Active bool
}

func NewPerson() Person {
	this := Person{}
	return this
}
