package converted

type Printable interface {
	Print()
}

type Person struct {
	Name string
	Age  int
}

var _ Printable = &Person{}

func NewPerson() Person {
	this := Person{}
	return this
}

func (this *Person) Print() {
	System.out.println(((("Person: " + name) + ", Age: ") + age))
}
