package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from simple_if_else.java:2:5
	if x {
		this.a()
	} else {
		this.b()
	}
}
