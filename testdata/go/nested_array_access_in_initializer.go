package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from nested_array_access_in_initializer.java:2:5
	items := []interface{}{obj.field, another.method()}
}
