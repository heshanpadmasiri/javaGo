package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) DoSomething() {
	System.out.println("Hello")
}
