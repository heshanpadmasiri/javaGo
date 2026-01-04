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
	// migrated from constructor_with_no_parameters.java:8:5
	p := NewPerson()
}
