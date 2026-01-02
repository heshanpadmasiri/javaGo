package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	alternatives := []Context{Context_START, Context_END}
}
