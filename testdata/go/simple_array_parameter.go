package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) processArray(data *[]int) {
	System.out.println(data)
}
