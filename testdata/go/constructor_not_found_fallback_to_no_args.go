package converted

type testConstructorNotFound struct {
}

func newTestConstructorNotFound() testConstructorNotFound {
	this := testConstructorNotFound{}
	return this
}

func (this *testConstructorNotFound) Test() {
	// FIXME: failed to find constructor for Date

	date := NewDate()
}
