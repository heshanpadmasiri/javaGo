package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) process(count int, data *[]int, name string) {
	// migrated from mixed_array_and_non_array_parameters.java:2:5
	System.out.println(count)
}
