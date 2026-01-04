package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from nested_if_else.java:2:5
	if x {
		if y {
			this.a()
		} else {
			this.b()
		}
	} else {
		this.c()
	}
}
