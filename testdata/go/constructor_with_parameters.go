package converted

type person struct {
	name string
	age  int
}

func NewPersonFromStringInt(name string, age int) person {
	this := person{}
	this.name = name
	this.age = age
	return this
}

func Test() {
	// migrated from constructor_with_parameters.java:10:5
	p := NewPersonFromStringInt("Alice", 30)
}
