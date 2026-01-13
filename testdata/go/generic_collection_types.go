package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) processList(items *[]string) {
	// migrated from generic_collection_types.java:2:5
	System.out.println(items)
}
