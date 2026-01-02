package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) processArray(data *[]int) {
	this.System.out.println(data)
}
