package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	if x {
		this.a()
	} else {
		this.b()
	}
}
