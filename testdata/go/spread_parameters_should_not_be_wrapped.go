package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) combine(numbers ...int) {
	this.System.out.println(numbers)
}
