package converted

type testConstructorNotFound struct {
}

func newTestConstructorNotFound() testConstructorNotFound {
	this := testConstructorNotFound{}
	return this
}

func (this *testConstructorNotFound) Test() {
	// migrated from constructor_not_found_fallback_to_no_args.java:4:5
	// FIXME: failed to find constructor for Date

	date := NewDate()
}
