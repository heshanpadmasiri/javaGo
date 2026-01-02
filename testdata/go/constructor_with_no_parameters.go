package converted

type person struct {
	name string
}

func NewPerson() person {
	this := person{}
	this.name = "Unknown"
	return this
}

func Test() {
	p := NewPerson()
}
