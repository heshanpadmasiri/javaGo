package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) combine(numbers ...int) {
	System.out.println(numbers)
}
