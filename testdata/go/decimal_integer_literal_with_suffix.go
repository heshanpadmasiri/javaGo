package converted

type test struct {
}

func newTest() test {
	this := test{}
	return this
}

func (this *test) test() {
	// migrated from decimal_integer_literal_with_suffix.java:2:5
	zero := int64(0)
	hundred := int64(100)
	lowercase := int64(0)
}
