package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) compare(arr1 *[]int, arr2 *[]string) {
	// migrated from multiple_array_parameters.java:2:5
	System.out.println("Comparing")
}
