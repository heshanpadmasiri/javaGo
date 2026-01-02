package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) DoSomething() {
	this.System.out.println("Hello")
}
