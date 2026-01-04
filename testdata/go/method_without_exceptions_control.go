package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) foo() {
	System.out.println("test")
}
