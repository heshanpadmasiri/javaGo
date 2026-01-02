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
	p := NewPersonFromStringInt("Alice", 30)
}
