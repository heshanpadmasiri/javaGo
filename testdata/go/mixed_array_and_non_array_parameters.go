package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) process(count int, data *[]int, name string) {
	this.System.out.println(count)
}
