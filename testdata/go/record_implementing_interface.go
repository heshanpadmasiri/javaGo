package converted

type Printable interface {
	Print()
}

type Person struct {
	Name string
	Age  int
}

var _ Printable = &Person{}

func (this *Person) Print() {
	this.System.out.println("Person: " + name + ", Age: " + age)
}
