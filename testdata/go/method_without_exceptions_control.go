package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) foo() {
	this.System.out.println("test")
}
