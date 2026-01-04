package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) processArray(data *[]int) {
	// migrated from simple_array_parameter.java:2:5
	System.out.println(data)
}
