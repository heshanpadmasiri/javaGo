package converted

type testConstructorNotFound struct {
}

func (this *testConstructorNotFound) Test() {
	// FIXME: failed to find constructor for Date

	date := NewDate()
}
