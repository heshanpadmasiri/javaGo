package converted

type person struct {
	name string
	age  int
}

func CreateDefault() Person {
	// migrated from static_method_before_constructor.java:5:5
	return NewPersonFromStringInt("Unknown", 0)
}

func NewPersonFromStringInt(name string, age int) person {
	this := person{}
	this.name = name
	this.age = age
	return this
}
